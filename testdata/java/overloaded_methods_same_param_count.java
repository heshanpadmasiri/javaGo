class Processor {
    public void process(String s) {
        System.out.println("String: " + s);
    }
    
    public void process(Integer i) {
        System.out.println("Integer: " + i);
    }
    
    public void test() {
        this.process("test");
    }
}
