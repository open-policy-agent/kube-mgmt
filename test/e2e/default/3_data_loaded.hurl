GET https://localhost:8443/v1/data/default
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result.*" count == 1
jsonpath "$.result.data-include" exists

GET https://localhost:8443/v1/data/default/data-include
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result['include.json'].inKey" == "inValue"
