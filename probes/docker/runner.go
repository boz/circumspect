package docker

import "context"

type Runner interface {
	Result() interface{}
	Stop()
	Done() <-chan struct{}
	Err() error
}

type Operation func(context.Context) (interface{}, error)

func NewRunner(ctx context.Context, op Operation) Runner {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)

	r := &runner{
		op:     op,
		donech: make(chan struct{}),
		cancel: cancel,
		ctx:    ctx,
	}

	go r.run()

	return r
}

type runner struct {
	op     Operation
	result interface{}
	err    error
	donech chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
}

func (r *runner) Result() interface{} {
	return r.result
}

func (r *runner) Stop() {
	r.cancel()
}

func (r *runner) Done() <-chan struct{} {
	return r.donech
}

func (r *runner) Err() error {
	<-r.donech
	return r.err
}

func (r *runner) run() {
	defer close(r.donech)
	defer r.cancel()

	// todo: retry

	r.result, r.err = r.op(r.ctx)
}
