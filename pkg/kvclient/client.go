package kvclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "kvstore-benchmarker/internal/proto"
)

// Client wraps the gRPC KeyValueStore client
type Client struct {
	conn   *grpc.ClientConn
	client pb.KeyValueStoreClient
	mu     sync.RWMutex
}

// NewClient creates a new KeyValueStore client
func NewClient(targetAddress string) (*Client, error) {
	conn, err := grpc.Dial(targetAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", targetAddress, err)
	}

	client := pb.NewKeyValueStoreClient(conn)
	return &Client{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Get retrieves a value by key
func (c *Client) Get(ctx context.Context, key []byte) (*pb.GetResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	req := &pb.GetRequest{Key: key}
	return c.client.Get(ctx, req)
}

// Put stores a key-value pair
func (c *Client) Put(ctx context.Context, key, value []byte) (*pb.PutResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	req := &pb.PutRequest{Key: key, Value: value}
	return c.client.Put(ctx, req)
}

// Delete removes a key-value pair
func (c *Client) Delete(ctx context.Context, key []byte) (*pb.DeleteResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	req := &pb.DeleteRequest{Key: key}
	return c.client.Delete(ctx, req)
}

// ConnectionPool manages multiple gRPC connections
type ConnectionPool struct {
	clients []*Client
	mu      sync.RWMutex
	index   int
}

// NewConnectionPool creates a pool of KV store clients
func NewConnectionPool(targetAddress string, numConnections int) (*ConnectionPool, error) {
	clients := make([]*Client, numConnections)

	for i := 0; i < numConnections; i++ {
		client, err := NewClient(targetAddress)
		if err != nil {
			// Close any clients that were successfully created
			for j := 0; j < i; j++ {
				clients[j].Close()
			}
			return nil, fmt.Errorf("failed to create client %d: %w", i, err)
		}
		clients[i] = client
	}

	return &ConnectionPool{
		clients: clients,
		index:   0,
	}, nil
}

// GetClient returns the next client in round-robin fashion
func (p *ConnectionPool) GetClient() *Client {
	p.mu.Lock()
	defer p.mu.Unlock()

	client := p.clients[p.index]
	p.index = (p.index + 1) % len(p.clients)
	return client
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for _, client := range p.clients {
		if err := client.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// HealthCheck performs a health check on all connections
func (p *ConnectionPool) HealthCheck(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastErr error
	for i, client := range p.clients {
		// Try a simple get operation as health check
		_, err := client.Get(ctx, []byte("health_check"))
		if err != nil {
			lastErr = fmt.Errorf("client %d health check failed: %w", i, err)
		}
	}
	return lastErr
}
