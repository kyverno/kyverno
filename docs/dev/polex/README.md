# PolicyException Matching using Trie Data Structure

## Introduction

In Kubernetes, PolicyException is a Custom Resource Definition (CRD) that contains a list of Policy names and a subsequent list of Rule names for each Policy. Previously, the process of matching an incoming Policy against PolicyExceptions involved linearly looping over all possible PolicyExceptions, which can be inefficient as the number of PolicyExceptions grows.

To optimize this process, a new controller has been implemented that indexes the PolicyExceptions using a Trie data structure. This approach provides efficient searching and matching capabilities, even when wildcards (`*`) are used in the PolicyException rules.

## Why Trie is the Best Solution

### Efficient Prefix Matching

The Trie data structure is particularly well-suited for prefix matching. In the context of PolicyException matching, policies and rules often share common prefixes. By using a Trie, we can efficiently search for matches based on these prefixes.

The Trie allows for fast lookups by traversing the tree based on the characters in the search key. Each node in the Trie represents a character, and the edges between nodes represent the transitions between characters. This structure enables us to quickly narrow down the search space and find matching PolicyExceptions.

### Handling Wildcards

One of the challenges in PolicyException matching is the presence of wildcards (`*`). Wildcards allow for flexible matching patterns, where a single `*` can match any number of characters.

Using a HashMap or other traditional data structures would not be suitable for handling wildcards efficiently. However, the Trie data structure can elegantly handle wildcards by treating them as special characters.

In the provided code, when a `*` is encountered during the search process, the algorithm explores multiple paths in the Trie. It continues the search by considering the wildcard as a match for any character and also by skipping the wildcard and moving to the next character in the search key. This allows for matching PolicyExceptions that contain wildcards at various positions.

### Efficient Storage and Memory Usage

The Trie data structure provides efficient storage and memory usage compared to other alternatives. It avoids storing redundant information by sharing common prefixes among different PolicyExceptions.

In the provided code, each node in the Trie contains an array of child nodes, representing the possible characters at that position. This allows for fast transitions between characters without the need for additional memory overhead.

Moreover, the Trie only stores the necessary information required for matching PolicyExceptions. It does not store the entire PolicyException object at each node, but rather keeps references to the relevant PolicyException objects at the appropriate nodes. This approach minimizes memory usage while still allowing for quick access to the matching PolicyExceptions.

### Scalability and Performance

The Trie-based solution offers excellent scalability and performance characteristics. As the number of PolicyExceptions grows, the Trie structure remains efficient in terms of search time and memory usage.

The time complexity of searching for a matching PolicyException in the Trie is proportional to the length of the search key, which is typically much smaller than the total number of PolicyExceptions. This means that the search operation remains fast, even with a large number of PolicyExceptions.

Additionally, the Trie supports efficient insertion and deletion operations. Inserting a new PolicyException into the Trie has a time complexity proportional to the length of the PolicyException key. Deleting a PolicyException from the Trie also has a similar time complexity, as it involves traversing the Trie based on the key and removing the appropriate references.

## Implementation Details

### Insertion
When inserting a new `PolicyException` into the Trie, we create a path from the root to a node representing the end of the string, with each character of the string being a node in the Trie. If a node for a particular character does not exist, we create it. We then append the `PolicyException` to the list of exceptions (`polexes`) at the terminal node.

### Search
Searching involves traversing the Trie based on the characters of the key string. If a wildcard character is encountered, the search branches out to consider all possible matches at that level. By using a helper function (`searchHelper`), we recursively search for matches, accounting for wildcards and ensuring uniqueness with the `uniquePolexes` map.

### Deletion
Deletion is handled by navigating to the appropriate node in the Trie and removing the `PolicyException` from the `polexes` list of that node by matching the `UID`.

## Conclusion

In conclusion, the Trie-based solution is the best approach for PolicyException matching in Kubernetes due to its efficiency in prefix matching, ability to handle wildcards, efficient storage and memory usage, and scalability. By indexing PolicyExceptions using a Trie, the matching process becomes faster and more efficient compared to linear looping over all PolicyExceptions.

The provided code implementation demonstrates how the Trie is constructed, how PolicyExceptions are inserted and deleted, and how the search operation is performed to find matching PolicyExceptions. The use of the Trie data structure enables efficient matching, even in the presence of wildcards, making it the most suitable solution for PolicyException matching in Kubernetes.
