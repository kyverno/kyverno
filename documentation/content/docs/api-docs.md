+++
title = "API CRD docs Reference"
+++
<html lang="en"><head>
<meta http-equiv="content-type" content="text/html; charset=UTF-8"><!-- base href="https://raw.githubusercontent.com/b-entangled/kyverno/663_api_docs/documentation/index.html" -->
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
<link rel="stylesheet" href="Kyverno%20API_files/bootstrap.css" integrity="sha384-ggOyR0iXCbMQv3Xipma34MD+dH/1fQ784/j6cY/iJTQUOhcWr7x9JvoRxT2MZw1T" crossorigin="anonymous">
<title>Kyverno API</title>
<style>
.bg-blue {
color: #ffffff;
background-color: #1589dd;
}
</style>
</head>
<body>
<div class="container">
<nav class="navbar navbar-expand-lg navbar-dark bg-dark">
<a class="navbar-brand" href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#"><p><b>Packages : </b></p></a>
<ul style="list-style:none">
<li>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io%2fv1"><b style="color: white">kyverno.io/v1</b></a>
</li>
</ul>
</nav>
<h2 id="kyverno.io/v1">kyverno.io/v1</h2>
Resource Types:
<ul><li>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ClusterPolicy">ClusterPolicy</a>
</li><li>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ClusterPolicyViolation">ClusterPolicyViolation</a>
</li><li>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequest">GenerateRequest</a>
</li><li>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolation">PolicyViolation</a>
</li></ul>
<hr>
<h3 id="kyverno.io/v1.ClusterPolicy">ClusterPolicy
</h3>
<p>
</p><p>ClusterPolicy …</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
string</td>
<td>
<code>
kyverno.io/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
string
</td>
<td><code>ClusterPolicy</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Spec">
Spec
</a>
</em>
</td>
<td>
<p>Spec is the information to identify the policy</p>
<br>
<br>
<table class="table table-striped">
<tbody><tr>
<td>
<code>rules</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">
[]Rule
</a>
</em>
</td>
<td>
<p>Rules contains the list of rules to be applied to resources</p>
</td>
</tr>
<tr>
<td>
<code>validationFailureAction</code><br>
<em>
string
</em>
</td>
<td>
<p>ValidationFailureAction provides choice to enforce rules to resources during policy violations.
Default value is “audit”.</p>
</td>
</tr>
<tr>
<td>
<code>background</code><br>
<em>
bool
</em>
</td>
<td>
<p>Background provides choice for applying rules to existing resources.
Default value is “true”.</p>
</td>
</tr>
</tbody></table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyStatus">
PolicyStatus
</a>
</em>
</td>
<td>
<p>Status contains statistics related to policy</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.ClusterPolicyViolation">ClusterPolicyViolation
</h3>
<p>
</p><p>ClusterPolicyViolation represents cluster-wide violations</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
string</td>
<td>
<code>
kyverno.io/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
string
</td>
<td><code>ClusterPolicyViolation</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationSpec">
PolicyViolationSpec
</a>
</em>
</td>
<td>
<br>
<br>
<table class="table table-striped">
<tbody><tr>
<td>
<code>policy</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies name of the policy</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>rules</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ViolatedRule">
[]ViolatedRule
</a>
</em>
</td>
<td>
<p>Specifies list of violated rule</p>
</td>
</tr>
</tbody></table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationStatus">
PolicyViolationStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.GenerateRequest">GenerateRequest
</h3>
<p>
</p><p>GenerateRequest is a request to process generate rule</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
string</td>
<td>
<code>
kyverno.io/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
string
</td>
<td><code>GenerateRequest</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestSpec">
GenerateRequestSpec
</a>
</em>
</td>
<td>
<p>Spec is the information to identify the generate request</p>
<br>
<br>
<table class="table table-striped">
<tbody><tr>
<td>
<code>policy</code><br>
<em>
string
</em>
</td>
<td>
<p>Policy - The required field represents the name of the policy</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
<p>ResourceSpec is the information to identify the generate request</p>
</td>
</tr>
<tr>
<td>
<code>context</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestContext">
GenerateRequestContext
</a>
</em>
</td>
<td>
<p>Context …</p>
</td>
</tr>
</tbody></table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestStatus">
GenerateRequestStatus
</a>
</em>
</td>
<td>
<p>Status contains statistics related to generate request</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.PolicyViolation">PolicyViolation
</h3>
<p>
</p><p>PolicyViolation represents namespaced violations</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
string</td>
<td>
<code>
kyverno.io/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
string
</td>
<td><code>PolicyViolation</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationSpec">
PolicyViolationSpec
</a>
</em>
</td>
<td>
<br>
<br>
<table class="table table-striped">
<tbody><tr>
<td>
<code>policy</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies name of the policy</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>rules</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ViolatedRule">
[]ViolatedRule
</a>
</em>
</td>
<td>
<p>Specifies list of violated rule</p>
</td>
</tr>
</tbody></table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationStatus">
PolicyViolationStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.CloneFrom">CloneFrom
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Generation">Generation</a>)
</p>
<p>
</p><p>CloneFrom - location of the resource
which will be used as source when applying ‘generate’</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>namespace</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies resource namespace</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies name of the resource</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Condition">Condition
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Deny">Deny</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">Rule</a>)
</p>
<p>
</p><p>Condition defines the evaluation condition</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>key</code><br>
<em>
interface{}
</em>
</td>
<td>
<p>Key contains key to compare</p>
</td>
</tr>
<tr>
<td>
<code>operator</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ConditionOperator">
ConditionOperator
</a>
</em>
</td>
<td>
<p>Operator to compare against value</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br>
<em>
interface{}
</em>
</td>
<td>
<p>Value to be compared</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.ConditionOperator">ConditionOperator
(<code>string</code> alias)<p></p></h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Condition">Condition</a>)
</p>
<p>
</p><p>ConditionOperator defines the type for condition operator</p>
<p></p>
<h3 id="kyverno.io/v1.Deny">Deny
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Validation">Validation</a>)
</p>
<p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Condition">
[]Condition
</a>
</em>
</td>
<td>
<p>Specifies set of condition to deny validation</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.ExcludeResources">ExcludeResources
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">Rule</a>)
</p>
<p>
</p><p>ExcludeResources container resource description of the resources that are to be excluded from the applying the policy rule</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>UserInfo</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.UserInfo">
UserInfo
</a>
</em>
</td>
<td>
<p>Specifies user information</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceDescription">
ResourceDescription
</a>
</em>
</td>
<td>
<p>Specifies resources to which rule is excluded</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.GenerateRequestContext">GenerateRequestContext
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestSpec">GenerateRequestSpec</a>)
</p>
<p>
</p><p>GenerateRequestContext stores the context to be shared</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>userInfo</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.RequestInfo">
RequestInfo
</a>
</em>
</td>
<td>
<p>UserRequestInfo …</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.GenerateRequestSpec">GenerateRequestSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequest">GenerateRequest</a>)
</p>
<p>
</p><p>GenerateRequestSpec stores the request specification</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>policy</code><br>
<em>
string
</em>
</td>
<td>
<p>Policy - The required field represents the name of the policy</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
<p>ResourceSpec is the information to identify the generate request</p>
</td>
</tr>
<tr>
<td>
<code>context</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestContext">
GenerateRequestContext
</a>
</em>
</td>
<td>
<p>Context …</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.GenerateRequestState">GenerateRequestState
(<code>string</code> alias)<p></p></h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestStatus">GenerateRequestStatus</a>)
</p>
<p>
</p><p>GenerateRequestState defines the state of</p>
<p></p>
<h3 id="kyverno.io/v1.GenerateRequestStatus">GenerateRequestStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequest">GenerateRequest</a>)
</p>
<p>
</p><p>GenerateRequestStatus stores the status of generated request</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestState">
GenerateRequestState
</a>
</em>
</td>
<td>
<p>State represents state of the generate request</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br>
<em>
string
</em>
</td>
<td>
<p>Message - An optional field is the request status message</p>
</td>
</tr>
<tr>
<td>
<code>generatedResources</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
[]ResourceSpec
</a>
</em>
</td>
<td>
<p>This will track the resources that are generated by the generate Policy
Will be used during clean up resources</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Generation">Generation
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">Rule</a>)
</p>
<p>
</p><p>Generation describes which resources will be created when other resource is created</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ResourceSpec</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>synchronize</code><br>
<em>
bool
</em>
</td>
<td>
<p>To keep resources synchronized with source resource</p>
</td>
</tr>
<tr>
<td>
<code>data</code><br>
<em>
interface{}
</em>
</td>
<td>
<p>Data …</p>
</td>
</tr>
<tr>
<td>
<code>clone</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.CloneFrom">
CloneFrom
</a>
</em>
</td>
<td>
<p>To clone resource from other resource</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.MatchResources">MatchResources
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">Rule</a>)
</p>
<p>
</p><p>MatchResources contains resource description of the resources that the rule is to apply on</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>UserInfo</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.UserInfo">
UserInfo
</a>
</em>
</td>
<td>
<p>Specifies user information</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceDescription">
ResourceDescription
</a>
</em>
</td>
<td>
<p>Specifies resources to which rule is applied</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Mutation">Mutation
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">Rule</a>)
</p>
<p>
</p><p>Mutation describes the way how Mutating Webhook will react on resource creation</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>overlay</code><br>
<em>
interface{}
</em>
</td>
<td>
<p>Specifies overlay patterns</p>
</td>
</tr>
<tr>
<td>
<code>patches</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Patch">
[]Patch
</a>
</em>
</td>
<td>
<p>Specifies JSON Patch</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Patch">Patch
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Mutation">Mutation</a>)
</p>
<p>
</p><p>Patch declares patch operation for created object according to RFC 6902</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies path of the resource</p>
</td>
</tr>
<tr>
<td>
<code>op</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies operations supported by JSON Patch.
i.e:- add, replace and delete</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br>
<em>
interface{}
</em>
</td>
<td>
<p>Specifies the value to be applied</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Policy">Policy
</h3>
<p>
</p><p>Policy contains rules to be applied to created resources</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Spec">
Spec
</a>
</em>
</td>
<td>
<p>Spec is the information to identify the policy</p>
<br>
<br>
<table class="table table-striped">
<tbody><tr>
<td>
<code>rules</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">
[]Rule
</a>
</em>
</td>
<td>
<p>Rules contains the list of rules to be applied to resources</p>
</td>
</tr>
<tr>
<td>
<code>validationFailureAction</code><br>
<em>
string
</em>
</td>
<td>
<p>ValidationFailureAction provides choice to enforce rules to resources during policy violations.
Default value is “audit”.</p>
</td>
</tr>
<tr>
<td>
<code>background</code><br>
<em>
bool
</em>
</td>
<td>
<p>Background provides choice for applying rules to existing resources.
Default value is “true”.</p>
</td>
</tr>
</tbody></table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyStatus">
PolicyStatus
</a>
</em>
</td>
<td>
<p>Status contains statistics related to policy</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.PolicyStatus">PolicyStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ClusterPolicy">ClusterPolicy</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Policy">Policy</a>)
</p>
<p>
</p><p>PolicyStatus mostly contains statistics related to policy</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>averageExecutionTime</code><br>
<em>
string
</em>
</td>
<td>
<p>average time required to process the policy rules on a resource</p>
</td>
</tr>
<tr>
<td>
<code>violationCount</code><br>
<em>
int
</em>
</td>
<td>
<p>number of violations created by this policy</p>
</td>
</tr>
<tr>
<td>
<code>rulesFailedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of rules that failed</p>
</td>
</tr>
<tr>
<td>
<code>rulesAppliedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of rules that were applied</p>
</td>
</tr>
<tr>
<td>
<code>resourcesBlockedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of resources that were blocked for failing a validate, across all rules</p>
</td>
</tr>
<tr>
<td>
<code>resourcesMutatedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of resources that were successfully mutated, across all rules</p>
</td>
</tr>
<tr>
<td>
<code>resourcesGeneratedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of resources that were successfully generated, across all rules</p>
</td>
</tr>
<tr>
<td>
<code>ruleStatus</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.RuleStats">
[]RuleStats
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.PolicyViolationSpec">PolicyViolationSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ClusterPolicyViolation">ClusterPolicyViolation</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolation">PolicyViolation</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationTemplate">PolicyViolationTemplate</a>)
</p>
<p>
</p><p>PolicyViolationSpec describes policy behavior by its rules</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>policy</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies name of the policy</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>rules</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ViolatedRule">
[]ViolatedRule
</a>
</em>
</td>
<td>
<p>Specifies list of violated rule</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.PolicyViolationStatus">PolicyViolationStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ClusterPolicyViolation">ClusterPolicyViolation</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolation">PolicyViolation</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationTemplate">PolicyViolationTemplate</a>)
</p>
<p>
</p><p>PolicyViolationStatus provides information regarding policyviolation status
status:
LastUpdateTime : the time the policy violation was updated</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>lastUpdateTime</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>LastUpdateTime : the time the policy violation was updated</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.PolicyViolationTemplate">PolicyViolationTemplate
</h3>
<p>
</p><p>PolicyViolationTemplate stores the information regarinding the resources for which a policy failed to apply</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationSpec">
PolicyViolationSpec
</a>
</em>
</td>
<td>
<br>
<br>
<table class="table table-striped">
<tbody><tr>
<td>
<code>policy</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies name of the policy</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>rules</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ViolatedRule">
[]ViolatedRule
</a>
</em>
</td>
<td>
<p>Specifies list of violated rule</p>
</td>
</tr>
</tbody></table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationStatus">
PolicyViolationStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.RequestInfo">RequestInfo
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestContext">GenerateRequestContext</a>)
</p>
<p>
</p><p>RequestInfo contains permission info carried in an admission request</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>roles</code><br>
<em>
[]string
</em>
</td>
<td>
<p>Roles is a list of possible role send the request</p>
</td>
</tr>
<tr>
<td>
<code>clusterRoles</code><br>
<em>
[]string
</em>
</td>
<td>
<p>ClusterRoles is a list of possible clusterRoles send the request</p>
</td>
</tr>
<tr>
<td>
<code>userInfo</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#userinfo-v1-authentication">
Kubernetes authentication/v1.UserInfo
</a>
</em>
</td>
<td>
<p>UserInfo is the userInfo carried in the admission request</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.ResourceDescription">ResourceDescription
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ExcludeResources">ExcludeResources</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.MatchResources">MatchResources</a>)
</p>
<p>
</p><p>ResourceDescription describes the resource to which the PolicyRule will be applied.</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kinds</code><br>
<em>
[]string
</em>
</td>
<td>
<p>Specifies list of resource kind</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies name of the resource</p>
</td>
</tr>
<tr>
<td>
<code>namespaces</code><br>
<em>
[]string
</em>
</td>
<td>
<p>Specifies list of namespaces</p>
</td>
</tr>
<tr>
<td>
<code>selector</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>Specifies the set of selectors</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.ResourceSpec">ResourceSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestSpec">GenerateRequestSpec</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.GenerateRequestStatus">GenerateRequestStatus</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Generation">Generation</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationSpec">PolicyViolationSpec</a>)
</p>
<p>
</p><p>ResourceSpec information to identify the resource</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kind</code><br>
<em>
string
</em>
</td>
<td>
<p>(Required): Specifies resource kind</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br>
<em>
string
</em>
</td>
<td>
<p>(Optional): Specifies resource namespace</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>(Required): Specifies resource name</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Rule">Rule
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Spec">Spec</a>)
</p>
<p>
</p><p>Rule is set of mutation, validation and generation actions
for the single resource description</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Name - A required field represents rule name</p>
</td>
</tr>
<tr>
<td>
<code>match</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.MatchResources">
MatchResources
</a>
</em>
</td>
<td>
<p>(Optional): Specifies resources for which the rule has to be applied.
If it’s defined, “kind” inside MatchResources block is required.</p>
</td>
</tr>
<tr>
<td>
<code>exclude</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ExcludeResources">
ExcludeResources
</a>
</em>
</td>
<td>
<p>(Optional): Specifies resources for which rule can be excluded</p>
</td>
</tr>
<tr>
<td>
<code>preconditions</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Condition">
[]Condition
</a>
</em>
</td>
<td>
<p>(Optional): Allows controlling policy rule execution</p>
</td>
</tr>
<tr>
<td>
<code>mutate</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Mutation">
Mutation
</a>
</em>
</td>
<td>
<p>(Optional): Specifies patterns to mutate resources</p>
</td>
</tr>
<tr>
<td>
<code>validate</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Validation">
Validation
</a>
</em>
</td>
<td>
<p>(Optional): Specifies patterns to validate resources</p>
</td>
</tr>
<tr>
<td>
<code>generate</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Generation">
Generation
</a>
</em>
</td>
<td>
<p>(Optional): Specifies patterns to create additional resources</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.RuleStats">RuleStats
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyStatus">PolicyStatus</a>)
</p>
<p>
</p><p>RuleStats provides status per rule</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ruleName</code><br>
<em>
string
</em>
</td>
<td>
<p>Rule name</p>
</td>
</tr>
<tr>
<td>
<code>averageExecutionTime</code><br>
<em>
string
</em>
</td>
<td>
<p>average time require to process the rule</p>
</td>
</tr>
<tr>
<td>
<code>violationCount</code><br>
<em>
int
</em>
</td>
<td>
<p>number of violations created by this rule</p>
</td>
</tr>
<tr>
<td>
<code>failedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of rules that failed</p>
</td>
</tr>
<tr>
<td>
<code>appliedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of rules that were applied</p>
</td>
</tr>
<tr>
<td>
<code>resourcesBlockedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of resources for whom update/create api requests were blocked as the resource did not satisfy the policy rules</p>
</td>
</tr>
<tr>
<td>
<code>resourcesMutatedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of resources that were successfully mutated</p>
</td>
</tr>
<tr>
<td>
<code>resourcesGeneratedCount</code><br>
<em>
int
</em>
</td>
<td>
<p>Count of resources that were successfully generated</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Spec">Spec
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ClusterPolicy">ClusterPolicy</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Policy">Policy</a>)
</p>
<p>
</p><p>Spec describes policy behavior by its rules</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>rules</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">
[]Rule
</a>
</em>
</td>
<td>
<p>Rules contains the list of rules to be applied to resources</p>
</td>
</tr>
<tr>
<td>
<code>validationFailureAction</code><br>
<em>
string
</em>
</td>
<td>
<p>ValidationFailureAction provides choice to enforce rules to resources during policy violations.
Default value is “audit”.</p>
</td>
</tr>
<tr>
<td>
<code>background</code><br>
<em>
bool
</em>
</td>
<td>
<p>Background provides choice for applying rules to existing resources.
Default value is “true”.</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.UserInfo">UserInfo
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.ExcludeResources">ExcludeResources</a>, 
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.MatchResources">MatchResources</a>)
</p>
<p>
</p><p>UserInfo filter based on users</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>roles</code><br>
<em>
[]string
</em>
</td>
<td>
<p>Specifies list of namespaced role names</p>
</td>
</tr>
<tr>
<td>
<code>clusterRoles</code><br>
<em>
[]string
</em>
</td>
<td>
<p>Specifies list of cluster wide role names</p>
</td>
</tr>
<tr>
<td>
<code>subjects</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#subject-v1-rbac">
[]Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<p>Specifies list of subject names like users, user groups, and service accounts</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.Validation">Validation
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Rule">Rule</a>)
</p>
<p>
</p><p>Validation describes the way how Validating Webhook will check the resource on creation</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>message</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies message to be displayed on validation policy violation</p>
</td>
</tr>
<tr>
<td>
<code>pattern</code><br>
<em>
interface{}
</em>
</td>
<td>
<p>Specifies validation pattern</p>
</td>
</tr>
<tr>
<td>
<code>anyPattern</code><br>
<em>
[]interface{}
</em>
</td>
<td>
<p>Specifies list of validation patterns</p>
</td>
</tr>
<tr>
<td>
<code>deny</code><br>
<em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.Deny">
Deny
</a>
</em>
</td>
<td>
<p>Specifies conditions to deny validation</p>
</td>
</tr>
</tbody>
</table>
<hr>
<h3 id="kyverno.io/v1.ViolatedRule">ViolatedRule
</h3>
<p>
(<em>Appears on:</em>
<a href="https://htmlpreview.github.io/?https://github.com/b-entangled/kyverno/blob/663_api_docs/documentation/index.html#kyverno.io/v1.PolicyViolationSpec">PolicyViolationSpec</a>)
</p>
<p>
</p><p>ViolatedRule stores the information regarding the rule</p>
<p></p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies violated rule name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies violated rule type</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br>
<em>
string
</em>
</td>
<td>
<p>Specifies violation message</p>
</td>
</tr>
</tbody>
</table>
<hr>
</div>
<script src="Kyverno%20API_files/jquery-3.js" integrity="sha384-q8i/X+965DzO0rT7abK41JStQIAqVgRVzpbzo5smXKp4YfRvH+8abtTE1Pi6jizo" crossorigin="anonymous"></script>
<script src="Kyverno%20API_files/popper.js" integrity="sha384-UO2eT0CpHqdSJQ6hJty5KVphtPhzWj9WO1clHTMGa3JDZwrnQq4sF86dIHNDz0W1" crossorigin="anonymous"></script>
<script src="Kyverno%20API_files/bootstrap.js" integrity="sha384-JjSmVgyd0p3pXB1rRibZUAYoIIy6OrQ6VrjIEaFf/nJGzIxFDsf4x0xIM+B07jRM" crossorigin="anonymous"></script>


</body></html>