class Test {
    void test() {
        try {
            doSomething();
        } catch (Exception e) {
            handleError(e);
        } finally {
            cleanup();
        }
    }
}