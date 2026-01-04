package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from try_catch_with_finally_block.java:2:5
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(Exception); ok {
					this.handleError(e)
				} else {
					panic(r) // re-panic if it's not a handled exception
				}
			}
		}()
		this.doSomething()
	}()
	this.cleanup()

}
