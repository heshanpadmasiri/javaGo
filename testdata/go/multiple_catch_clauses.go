package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from multiple_catch_clauses.java:2:5
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(IllegalArgumentException); ok {
					this.handleIllegal(e)
				} else if _, ok := r.(IllegalStateException); ok {
					this.handleState(e)
				} else {
					panic(r) // re-panic if it's not a handled exception
				}
			}
		}()
		this.riskyOperation()
	}()

}
