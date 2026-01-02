import java.util.Date;

class TestConstructorNotFound {
    public void test() {
        // Date class exists but its constructor is not in the migration context
        // Should fall back to no-args constructor with FIXME
        Date date = new Date();
    }
}
