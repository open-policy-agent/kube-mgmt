GET https://localhost:8443/v1/data
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result.*" count == 0
