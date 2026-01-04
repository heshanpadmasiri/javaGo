package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) getCompletion(context ParserRuleContext, nextToken Token) Solution {
	// migrated from single_catch_clause_with_illegalstateexception.java:2:5
	tempCtxStack := this.ctxStack
	this.ctxStack = this.getCtxStackSnapshot()
	var sol Solution
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(IllegalStateException); ok {
					if false {
						panic("assertion failed")
					}
					sol = this.getResolution(context, nextToken)
				} else {
					panic(r) // re-panic if it's not a handled exception
				}
			}
		}()
		sol = this.getInsertSolution(context)
	}()

	this.ctxStack = tempCtxStack
	return sol
}
