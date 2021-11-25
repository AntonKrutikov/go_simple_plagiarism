Description:

API endpoint to verify target URL has occurrence of passed phrase using fuzzy match.

Returns JSON response with result or errors.

__If target string can't be founded in text - return response with enmpty foundText value__

Default endpoint address: http://localhost:8080/api/search

Available REQUEST params (GET and POST both supported):
- url: "string" (required, must be valid url with scheme)
- search: "string" (required)
- count_before: "int" (optional, default=0)
- count_after: "int" (optional, default=0)
- count_before: "int" (optional, default=0)
- fuzzy_distance: "int" (optional, default=20). Max char count skip in one fuzzy search iteration.

Responses:

- StatusCode: 200

        {
            "searchRequest":"working with him s",
            "url":"https://www.upwork.com/cat/sales-and-marketing","before":"",
            "foundText":"",
            "after":""
        }



- StatusCode: 400, 404, 500

        {
            "error":"Error text",
            "innerError":"Inner error text"
        }

Supported errors:
- Missing URL or SEARCH parameter - 400
- Wrong URL format - 400
- Can't complete request without proxy and than with proxy - 404
- Unexpected errors - 500


Endpoint for manual testing http://localhost:8080/test