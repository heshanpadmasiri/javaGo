# Agent instructions

## Code style

- Prefer switch statements to if-else statements where possible
- If you have very complex if else block or switch case block (have > 5 lines), consider extracting that to a function with a meaningful name
- Don't get side tracked trying to fix TODO/FIXME comments if they are not relevant to the problem you are trying to solve

## Migration strategies

- See [README](README.md)

## Tasks

### Adding support for new grammar rules

1. Add a unit tests with expected java source and expected go source. _At this stage we expect test to fail_
2. Add logs to the source where needed to figure out what s-expressions you are going to get.
3. Based on that implement the logic to convert and generate the go source. _At this point we expect the tests to pass_
   Then remove any logs you have added

### Updating tests

- Run tests with `-update` flag
