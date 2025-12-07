# Adding support for new grammar rules

1. Add a unit tests with expected java source and expected go source. _At this stage we expect test to fail_
2. Add logs to the source where needed to figure out what s-expressions you are going to get.
3. Based on that implement the logic to convert and generate the go source. _At this point we expect the tests to pass_
   Then remove any logs you have added
