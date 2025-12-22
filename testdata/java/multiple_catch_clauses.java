class Test {
    void test() {
        try {
            riskyOperation();
        } catch (IllegalArgumentException e) {
            handleIllegal(e);
        } catch (IllegalStateException e) {
            handleState(e);
        }
    }
}