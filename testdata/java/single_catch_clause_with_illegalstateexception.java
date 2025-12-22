class Test {
    Solution getCompletion(ParserRuleContext context, STToken nextToken) {
        ArrayDeque<ParserRuleContext> tempCtxStack = this.ctxStack;
        this.ctxStack = getCtxStackSnapshot();

        Solution sol;
        try {
            sol = getInsertSolution(context);
        } catch (IllegalStateException exception) {
            assert false : "Oh no, something went bad";
            sol = getResolution(context, nextToken);
        }

        this.ctxStack = tempCtxStack;
        return sol;
    }
}