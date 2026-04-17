GET http://localhost:8080/v1/policies
HTTP 200
[Asserts]
jsonpath "$.result" count == 2
jsonpath "$.result[?(@.id == 'default/multi-file-policy/a.rego')]" count >= 1
jsonpath "$.result[?(@.id == 'default/multi-file-policy/b.rego')]" count >= 1

POST http://localhost:8080/v1/data/my_pkg/my_rule
Content-Type: application/json
{"input": {"hello": "world"}}

HTTP 200
[Asserts]
jsonpath "$.result" == true

POST http://localhost:8080/v1/data/my_pkg/my_rule
Content-Type: application/json
{"input": {"hello": "incorrect"}}

HTTP 200
[Asserts]
jsonpath "$.result" == false
