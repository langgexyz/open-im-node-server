package registry

import (
	"context"
	"strings"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

const etcdPrefix = "/open-im/biz/"

// Registry maintains an in-memory routing table: service_name → backend_addr
type Registry struct {
	mu     sync.RWMutex
	routes map[string]string
}

// New creates a new empty Registry.
func New() *Registry {
	return &Registry{routes: make(map[string]string)}
}

// Set stores or updates the backend address for the given service name.
func (r *Registry) Set(name, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[name] = addr
}

// Delete removes the route for the given service name.
func (r *Registry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, name)
}

// Get returns the backend address for the given service name.
func (r *Registry) Get(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	addr, ok := r.routes[name]
	return addr, ok
}

// Watch subscribes to the etcd /open-im/biz/ prefix and syncs changes into the
// in-memory routing table. It blocks until ctx is cancelled; run it in a goroutine.
func (r *Registry) Watch(ctx context.Context, client *clientv3.Client) {
	// Initial load of all existing keys.
	resp, err := client.Get(ctx, etcdPrefix, clientv3.WithPrefix())
	if err == nil {
		for _, kv := range resp.Kvs {
			name := strings.TrimPrefix(string(kv.Key), etcdPrefix)
			r.Set(name, string(kv.Value))
		}
	}

	// Stream watch events.
	ch := client.Watch(ctx, etcdPrefix, clientv3.WithPrefix())
	for wresp := range ch {
		for _, ev := range wresp.Events {
			name := strings.TrimPrefix(string(ev.Kv.Key), etcdPrefix)
			switch ev.Type {
			case clientv3.EventTypePut:
				r.Set(name, string(ev.Kv.Value))
			case clientv3.EventTypeDelete:
				r.Delete(name)
			}
		}
	}
}

// NewEtcdClient creates an etcd v3 client. addr example: "127.0.0.1:2379".
func NewEtcdClient(addr string) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints: []string{addr},
		Logger:    zap.NewNop(),
	})
}
