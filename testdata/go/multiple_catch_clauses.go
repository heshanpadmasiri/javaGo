package converted

type test struct {
}

func (this *test) test() {
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
