package converted

type test struct {
}

func (this *test) test() {
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
