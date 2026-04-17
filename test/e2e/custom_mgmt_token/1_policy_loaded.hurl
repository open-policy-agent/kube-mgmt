GET https://localhost:8443/v1/policies
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result[?(@.id == 'default/policy-include/include.rego')]" count >= 1

GET https://localhost:8443/v1/data/example/include/allow
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result" == true
