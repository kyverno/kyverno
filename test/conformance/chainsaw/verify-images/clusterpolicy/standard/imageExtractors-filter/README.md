# imageExtractors with filter field

This test verifies that the `filter` field on imageExtractors correctly selects
only the array element where `key` matches the filter value during wildcard
path expansion. Without the filter, non-image params like `{"name":"kind","value":"Task"}`
would fail OCI reference parsing and abort extraction.
