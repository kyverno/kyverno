package e2e

type testData struct {
	group, version, resource, namespace string
	manifest                            []byte
}

// Pod CPU hog test
var PodCPUHogTest = struct {
	//TestName - Name of the Test
	TestName string
	TestData []testData
}{

	TestName: "test-litmus-chaos-experiment",
	TestData: []testData{
		{
			group:     "",
			version:   "v1",
			resource:  "Pod",
			namespace: "test-litmus",
			manifest:  KyvernoTestResourcesYaml,
		},
	},
}
