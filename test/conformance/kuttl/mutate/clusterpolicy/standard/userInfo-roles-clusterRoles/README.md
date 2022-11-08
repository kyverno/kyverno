# Title

This test verifies that Kyverno is able to pick up and write the `request.userInfo` information from the AdmissionReview payload correctly, as well as the pre-defined vars `request.roles` and `request.clusterRoles` by creating and then performing an action as a new user in the system. The expectation is the custom group and username are both being reflected correctly in a mutation. Similar tests exist for validation flows.
