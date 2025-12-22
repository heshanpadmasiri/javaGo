package converted

type test struct {
}

func (this *test) calculate() int {
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
