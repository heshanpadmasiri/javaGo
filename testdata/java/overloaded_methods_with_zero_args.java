class Runner {
    public void run() {
        System.out.println("Running once");
    }
    
    public void run(int times) {
        for (int i = 0; i < times; i++) {
            System.out.println("Running iteration " + i);
        }
    }
    
    public void test() {
        this.run();
        this.run(5);
    }
}
