GET http://localhost:8080/v1/data/kubernetes/services/ignore-me
HTTP 200
[Asserts]
jsonpath "$.result" not exists

GET http://localhost:8080/v1/data/kubernetes/services/dont-ignore-me
HTTP 200
[Asserts]
jsonpath "$.result.*" count == 1
jsonpath "$.result.dont-ignore-me" exists

GET http://localhost:8080/v1/data/kubernetes/services/default
HTTP 200
[Asserts]
jsonpath "$.result.*" count == 2
