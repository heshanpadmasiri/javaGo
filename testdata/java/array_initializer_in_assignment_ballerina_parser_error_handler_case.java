class Test {
    void test(ParserRuleContext parentCtx) {
        ParserRuleContext[] alternatives = null;
        switch (parentCtx) {
            case ARG_LIST:
                alternatives = new ParserRuleContext[] { ParserRuleContext.COMMA,
                        ParserRuleContext.BINARY_OPERATOR,
                        ParserRuleContext.ARG_LIST_END };
                break;
        }
    }
}

