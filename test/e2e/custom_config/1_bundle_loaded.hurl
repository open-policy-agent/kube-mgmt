GET https://localhost:8443/v1/data
Authorization: Bearer {{token}}

HTTP 200
[Asserts]
jsonpath "$.result.test_helm_kubernetes_quickstart.*" count == 3
