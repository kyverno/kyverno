# Title

Checks that a ClusterPolicy without defining an imageExtractor causes a CustomResource to pass through. Since the ClusterPolicy does not name a field from which to extract the image, no verification can be performed. Expected result is the Task is created even though the image within is not verified.