package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test(parentCtx ParserRuleContext) {
	// migrated from array_initializer_in_assignment_ballerina_parser_error_handler_case.java:2:5
	alternatives := nil
	switch parentCtx {
	case ARG_LIST:
		alternatives = []ParserRuleContext{ParserRuleContext_COMMA, ParserRuleContext_BINARY_OPERATOR, ParserRuleContext_ARG_LIST_END}
		break
	}
}
