reviewed: true

## 1. Establish compatibility

- [x] 1.1 Check module publication, compatibility policy, repository references, and known downstream consumers
- [x] 1.2 Record that external use cannot be ruled out for the public module path

## 2. Apply the safe decision

- [x] 2.1 Keep `StripAnsi` exported because compatibility is uncertain
- [x] 2.2 Record no removal: this public API cleanup is not safe
- [x] 2.3 No source change required, so no additional test run is needed
