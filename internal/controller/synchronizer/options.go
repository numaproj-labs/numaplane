package synchronizer

type options struct {
	// Number of workers.
	workers int
	// Time in milliseconds, each element in the work queue will be picked up in an interval of this period of time.
	taskInterval int
}

type Option func(*options)

func defaultOptions() *options {
	return &options{
		workers:      50,
		taskInterval: 120000,
	}
}

// WithWorkers sets the number of workers working on the synchronization.
func WithWorkers(n int) Option {
	return func(o *options) {
		o.workers = n
	}
}

// WithTaskInterval sets the interval of picking up a task from the work queue.
func WithTaskInterval(n int) Option {
	return func(o *options) {
		o.taskInterval = n
	}
}
