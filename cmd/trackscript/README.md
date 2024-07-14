# trackscript

## Examples

Play a, then a and b together.
```
bpm 120
pat aaa a.abc
pat bbb b.mid
aaa
aaa | bbb
```

Autodeclare patterns based on filename:
```
a
a | b
```

Create phrases from patterns:
```
phrase ac { a a c c }
phrase ba { b b a a }
ac ac ac ac
ba ba ba ba
```