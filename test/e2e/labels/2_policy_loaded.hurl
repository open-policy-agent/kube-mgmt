GET http://localhost:8080/v1/policies
HTTP 200
[Asserts]
jsonpath "$.result[?(@.id == 'default/policy-include/include.rego')]" count >= 1

GET http://localhost:8080/v1/data/example/include/allow
HTTP 200
[Asserts]
jsonpath "$.result" == true
