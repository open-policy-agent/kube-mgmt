GET http://localhost:8080/v1/data/my_pkg/my_rule
HTTP 200
[Asserts]
jsonpath "$.result" not exists
