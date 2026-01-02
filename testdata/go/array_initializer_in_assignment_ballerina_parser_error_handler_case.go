package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test(parentCtx ParserRuleContext) {
	alternatives := nil
	switch parentCtx {
	case ARG_LIST:
		alternatives = []ParserRuleContext{ParserRuleContext_COMMA, ParserRuleContext_BINARY_OPERATOR, ParserRuleContext_ARG_LIST_END}
		break
	}
}
