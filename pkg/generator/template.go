package generator

import "text/template"

var tpl = template.Must(template.New("generated").
	Funcs(template.FuncMap{
		"lcFirst": lcFirst,
	}).
	Parse(`
// Code generated by github.com/gallery-so/dataloaden, DO NOT EDIT.

package {{.Package}}

import (
    "sync"
    "time"
	"context"

    {{if .KeyType.ImportPath}}"{{.KeyType.ImportPath}}"{{end}}
    {{if .ValType.ImportPath}}"{{.ValType.ImportPath}}"{{end}}
)

type {{.Name}}Settings interface {
	getContext() context.Context
	getWait() time.Duration
	getMaxBatchOne() int
	getMaxBatchMany() int
	getDisableCaching() bool
	getPublishResults() bool
	getPreFetchHook() func(context.Context, string) context.Context
	getPostFetchHook() func(context.Context, string)
	getSubscriptionRegistry() *[]interface{}
	getMutexRegistry() *[]*sync.Mutex
}

{{- if not .ValType.IsSlice }}
// {{.Name}}CacheSubscriptions 
type {{.Name}}CacheSubscriptions struct {
	// AutoCacheWithKey is a function that returns the {{.KeyType.String}} cache key for a {{.ValType.StringWithoutModifiers}}.
	// If AutoCacheWithKey is not nil, this loader will automatically cache published results from other loaders
	// that return a {{.ValType.StringWithoutModifiers}}. Loaders that return pointers or slices of {{.ValType.StringWithoutModifiers}}
	// will be dereferenced/iterated automatically, invoking this function with the base {{.ValType.StringWithoutModifiers}} type.
	AutoCacheWithKey func({{.ValType.StringWithoutModifiers}}) {{.KeyType.String}}

	// AutoCacheWithKeys is a function that returns the []{{.KeyType.String}} cache keys for a {{.ValType.StringWithoutModifiers}}.
	// Similar to AutoCacheWithKey, but for cases where a single value gets cached by many keys.
	// If AutoCacheWithKeys is not nil, this loader will automatically cache published results from other loaders
	// that return a {{.ValType.StringWithoutModifiers}}. Loaders that return pointers or slices of {{.ValType.StringWithoutModifiers}}
	// will be dereferenced/iterated automatically, invoking this function with the base {{.ValType.StringWithoutModifiers}} type.
	AutoCacheWithKeys func({{.ValType.StringWithoutModifiers}}) []{{.KeyType.String}}

	// TODO: Allow custom cache functions once we're able to use generics. It could be done without generics, but
	// would be messy and error-prone. A non-generic implementation might look something like:
	//
	//   CustomCacheFuncs []func(primeFunc func(key, value)) func(typeToRegisterFor interface{})
	//
	// where each CustomCacheFunc is a closure that receives this loader's unsafePrime method and returns a
	// function that accepts the type it's registering for and uses that type and the unsafePrime method
	// to prime the cache.
}
{{- end }}

func (l *{{.Name}}) setContext(ctx context.Context) {
	l.ctx = ctx
}

func (l *{{.Name}}) setWait(wait time.Duration) {
	l.wait = wait
}

func (l *{{.Name}}) setMaxBatch(maxBatch int) {
	l.maxBatch = maxBatch
}

func (l *{{.Name}}) setDisableCaching(disableCaching bool) {
	l.disableCaching = disableCaching
}

func (l *{{.Name}}) setPublishResults(publishResults bool) {
	l.publishResults = publishResults
}

func (l *{{.Name}}) setPreFetchHook(preFetchHook func(context.Context, string) context.Context) {
	l.preFetchHook = preFetchHook
}

func (l *{{.Name}}) setPostFetchHook(postFetchHook func(context.Context, string)) {
	l.postFetchHook = postFetchHook
}

// New{{.Name}} creates a new {{.Name}} with the given settings, functions, and options
func New{{.Name}}(
	settings {{.Name}}Settings, fetch func(ctx context.Context, keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error),
	{{- if not .ValType.IsSlice }}
	funcs {{.Name}}CacheSubscriptions,
	{{- end }}
	opts ...func(interface{
		setContext(context.Context)
		setWait(time.Duration)
		setMaxBatch(int)
		setDisableCaching(bool)
		setPublishResults(bool)
		setPreFetchHook(func(context.Context, string) context.Context)
		setPostFetchHook(func(context.Context, string))
	}),
	) *{{.Name}} {
	loader := &{{.Name}}{
		ctx: settings.getContext(),
		wait: settings.getWait(),
		disableCaching: settings.getDisableCaching(),
		publishResults: settings.getPublishResults(),
		preFetchHook: settings.getPreFetchHook(),
		postFetchHook: settings.getPostFetchHook(),
		subscriptionRegistry: settings.getSubscriptionRegistry(),
		mutexRegistry: settings.getMutexRegistry(),
		{{- if .ValType.IsSlice }}
		maxBatch: settings.getMaxBatchMany(),
		{{- else }}
		maxBatch: settings.getMaxBatchOne(),
		{{- end }}
	}

	for _, opt := range opts {
		opt(loader)
	}

	// Set this after applying options, in case a different context was set via options
	loader.fetch = func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error) {
		ctx := loader.ctx

		// Allow the preFetchHook to modify and return a new context
		if loader.preFetchHook != nil {
			ctx = loader.preFetchHook(ctx, "{{.Name}}")
		}

		results, errors := fetch(ctx, keys)

		if loader.postFetchHook != nil {
			loader.postFetchHook(ctx, "{{.Name}}")
		}
		
		return results, errors
	}

	if loader.subscriptionRegistry == nil {
		panic("subscriptionRegistry may not be nil")
	}

	if loader.mutexRegistry == nil {
		panic("mutexRegistry may not be nil")
	}

	{{ if .ValType.IsSlice }}
	// No cache functions here; caching isn't very useful for dataloaders that return slices. This dataloader can
	// still send its results to other cache-priming receivers, but it won't register its own cache-priming function.
	{{- else }}
	if !loader.disableCaching {
		// One-to-one mappings: cache one value with one key
		if funcs.AutoCacheWithKey != nil {
			cacheFunc := func(t {{.ValType.StringWithoutModifiers}}) {
				loader.unsafePrime(funcs.AutoCacheWithKey(t), t)
			}
			loader.registerCacheFunc(&cacheFunc, &loader.mu)
		}

		// One-to-many mappings: cache one value with many keys
		if funcs.AutoCacheWithKeys != nil {
			cacheFunc := func(t {{.ValType.StringWithoutModifiers}}) {
				keys := funcs.AutoCacheWithKeys(t)
				for _, key := range keys {
					loader.unsafePrime(key, t)
				}
			}
			loader.registerCacheFunc(&cacheFunc, &loader.mu)
		}
	}
	{{- end }}

	return loader
}

// {{.Name}} batches and caches requests          
type {{.Name}} struct {
	// context passed to fetch functions
	ctx context.Context

	// this method provides the data for the loader
	fetch func(keys []{{.KeyType.String}}) ([]{{.ValType.String}}, []error)

	// how long to wait before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	// whether this dataloader will cache results
	disableCaching bool

	// whether this dataloader will publish its results for others to cache
	publishResults bool

	// a hook invoked before the fetch operation, useful for things like tracing.
	// the returned context will be passed to the fetch operation.
	preFetchHook func(ctx context.Context, loaderName string) context.Context

	// a hook invoked after the fetch operation, useful for things like tracing
	postFetchHook func(ctx context.Context, loaderName string)

	// a shared slice where dataloaders will register and invoke caching functions.
	// the same slice should be passed to every dataloader.
	subscriptionRegistry *[]interface{}

	// a shared slice, parallel to the subscription registry, that holds a reference to the
	// cache mutex for the subscription's dataloader
	mutexRegistry *[]*sync.Mutex

	// INTERNAL

	// lazily created cache
	cache map[{{.KeyType.String}}]{{.ValType.String}}

	// typed cache functions
	//subscribers []func({{.ValType.String}})
	subscribers []{{.Name|lcFirst}}Subscriber

	// functions used to cache published results from other dataloaders
	cacheFuncs []interface{}

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *{{.Name|lcFirst}}Batch

	// mutex to prevent races
	mu sync.Mutex

	// only initialize our typed subscription cache once
	once sync.Once
}

type {{.Name|lcFirst}}Batch struct {
	keys    []{{.KeyType}}
	data    []{{.ValType.String}}
	error   []error
	closing bool
	done    chan struct{}
}

// Load a {{.ValType.Name}} by key, batching and caching will be applied automatically
func (l *{{.Name}}) Load(key {{.KeyType.String}}) ({{.ValType.String}}, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a {{.ValType.Name}}.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadThunk(key {{.KeyType.String}}) func() ({{.ValType.String}}, error) {
	l.mu.Lock()
	if !l.disableCaching {
		if it, ok := l.cache[key]; ok {
			l.mu.Unlock()
			return func() ({{.ValType.String}}, error) {
				return it, nil
			}
		}
	}
	if l.batch == nil {
		l.batch = &{{.Name|lcFirst}}Batch{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)
	l.mu.Unlock()

	return func() ({{.ValType.String}}, error) {
		<-batch.done

		var data {{.ValType.String}}
		if pos < len(batch.data) {
			data = batch.data[pos]
		}

		var err error
		// its convenient to be able to return a single error for everything
		if len(batch.error) == 1 {
			err = batch.error[0]
		} else if batch.error != nil {
			err = batch.error[pos]
		}

		if err == nil {
			if !l.disableCaching {
				l.mu.Lock()
				l.unsafeSet(key, data)
				l.mu.Unlock()
			}

			if l.publishResults {
				l.publishToSubscribers(data)
			}
		}

		return data, err
	}
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *{{.Name}}) LoadAll(keys []{{.KeyType}}) ([]{{.ValType.String}}, []error) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
	}
	return {{.ValType.Name|lcFirst}}s, errors
}

// LoadAllThunk returns a function that when called will block waiting for a {{.ValType.Name}}s.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *{{.Name}}) LoadAllThunk(keys []{{.KeyType}}) (func() ([]{{.ValType.String}}, []error)) {
	results := make([]func() ({{.ValType.String}}, error), len(keys))
 	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}
	return func() ([]{{.ValType.String}}, []error) {
		{{.ValType.Name|lcFirst}}s := make([]{{.ValType.String}}, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			{{.ValType.Name|lcFirst}}s[i], errors[i] = thunk()
		}
		return {{.ValType.Name|lcFirst}}s, errors
	}
}

// Prime the cache with the provided key and value. If the key already exists, no change is made
// and false is returned.
// (To forcefully prime the cache, clear the key first with loader.clear(key).prime(key, value).)
func (l *{{.Name}}) Prime(key {{.KeyType}}, value {{.ValType.String}}) bool {
	if l.disableCaching {
		return false
	}
	l.mu.Lock()
	var found bool
	if _, found = l.cache[key]; !found {
		{{- if .ValType.IsPtr }}
			// make a copy when writing to the cache, its easy to pass a pointer in from a loop var
			// and end up with the whole cache pointing to the same value.
			cpy := *value
			l.unsafeSet(key, &cpy)
		{{- else if .ValType.IsSlice }}
			// make a copy when writing to the cache, its easy to pass a pointer in from a loop var
			// and end up with the whole cache pointing to the same value.
			cpy := make({{.ValType.String}}, len(value))
			copy(cpy, value)
			l.unsafeSet(key, cpy)
		{{- else }}
			l.unsafeSet(key, value)
		{{- end }}
	}
	l.mu.Unlock()
	return !found
}

{{- if not .ValType.IsSlice }}
// Prime the cache without acquiring locks. Should only be used when the lock is already held.
func (l *{{.Name}}) unsafePrime(key {{.KeyType}}, value {{.ValType.StringWithoutModifiers}}) bool {
	if l.disableCaching {
		return false
	}
	var found bool
	if _, found = l.cache[key]; !found {
		{{- if .ValType.IsPtr }}
			l.unsafeSet(key, &value)
		{{- else }}
			l.unsafeSet(key, value)
		{{- end }}
	}
	return !found
}
{{- end }}

// Clear the value at key from the cache, if it exists
func (l *{{.Name}}) Clear(key {{.KeyType}}) {
	if l.disableCaching {
		return
	}
	l.mu.Lock()
	delete(l.cache, key)
	l.mu.Unlock()
}

func (l *{{.Name}}) unsafeSet(key {{.KeyType}}, value {{.ValType.String}}) {
	if l.cache == nil {
		l.cache = map[{{.KeyType}}]{{.ValType.String}}{}
	}
	l.cache[key] = value
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *{{.Name|lcFirst}}Batch) keyIndex(l *{{.Name}}, key {{.KeyType}}) int {
	for i, existingKey := range b.keys {
		if key == existingKey {
			return i
		}
	}

	pos := len(b.keys)
	b.keys = append(b.keys, key)
	if pos == 0 {
		go b.startTimer(l)
	}

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		if !b.closing {
			b.closing = true
			l.batch = nil
			go b.end(l)
		}
	}

	return pos
}

func (b *{{.Name|lcFirst}}Batch) startTimer(l *{{.Name}}) {
	time.Sleep(l.wait)
	l.mu.Lock()

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		l.mu.Unlock()
		return
	}

	l.batch = nil
	l.mu.Unlock()

	b.end(l)
}

func (b *{{.Name|lcFirst}}Batch) end(l *{{.Name}}) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}

type {{.Name|lcFirst}}Subscriber struct {
	cacheFunc func({{.ValType.StringWithoutModifiers}})
	mutex *sync.Mutex
}

func (l *{{.Name}}) publishToSubscribers(value {{.ValType.String}}) {
	// Lazy build our list of typed cache functions once
	l.once.Do(func() {
		for i, subscription := range *l.subscriptionRegistry {
			if typedFunc, ok := subscription.(*func({{.ValType.StringWithoutModifiers}})); ok {
				// Don't invoke our own cache function
				if !l.ownsCacheFunc(typedFunc) {
					l.subscribers = append(l.subscribers, {{.Name|lcFirst}}Subscriber{cacheFunc: *typedFunc, mutex: (*l.mutexRegistry)[i]})
				}
			}
		}
	})

	// Handling locking here (instead of in the subscribed functions themselves) isn't the
	// ideal pattern, but it's an optimization that allows the publisher to iterate over slices
	// without having to acquire the lock many times.
	for _, s := range l.subscribers {
		s.mutex.Lock()
		{{- if .ValType.IsSliceOfPtrs }}
		for _, v := range value {
			if v != nil {
				s.cacheFunc(*v)
			}
		}
		{{- else if .ValType.IsPtr }}
		if value != nil {
			s.cacheFunc(*value)
		}
		{{- else if .ValType.IsSlice }}
		for _, v := range value {
			s.cacheFunc(v)
		}
		{{- else }}
		s.cacheFunc(value)
		{{- end }}
		s.mutex.Unlock()
	}
}

func (l *{{.Name}}) registerCacheFunc(cacheFunc interface{}, mutex *sync.Mutex) {
	l.cacheFuncs = append(l.cacheFuncs, cacheFunc)
	*l.subscriptionRegistry = append(*l.subscriptionRegistry, cacheFunc)
	*l.mutexRegistry = append(*l.mutexRegistry, mutex)
}

func (l *{{.Name}}) ownsCacheFunc(f *func({{.ValType.StringWithoutModifiers}})) bool {
	for _, cacheFunc := range l.cacheFuncs {
		if cacheFunc == f {
			return true
		}
	}

	return false
}
`))
