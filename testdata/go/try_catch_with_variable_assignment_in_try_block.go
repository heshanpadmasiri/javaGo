package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) calculate() int {
	// migrated from try_catch_with_variable_assignment_in_try_block.java:2:5
	var result int
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(RuntimeException); ok {
					result = this.defaultValue()
				} else {
					panic(r) // re-panic if it's not a handled exception
				}
			}
		}()
		result = this.compute()
	}()

	return result
}
