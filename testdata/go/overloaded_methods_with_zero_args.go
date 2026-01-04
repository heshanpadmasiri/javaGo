package converted

type runner struct {
}

func newRunner() runner {
	this := runner{}
	return this
}

func (this *runner) Run() {
	// migrated from overloaded_methods_with_zero_args.java:2:5
	System.out.println("Running once")
}

func (this *runner) RunWithInt(times int) {
	// migrated from overloaded_methods_with_zero_args.java:6:5
	i := 0
	for ; i < times; i++ {
		System.out.println(("Running iteration " + i))
	}
}

func (this *runner) Test() {
	// migrated from overloaded_methods_with_zero_args.java:12:5
	this.Run()
	this.RunWithInt(5)
}
