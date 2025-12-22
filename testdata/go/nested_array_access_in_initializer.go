package converted

type test struct {
}

func (this *test) test() {
	items := []interface{}{obj.field, this.another.method()}
}
