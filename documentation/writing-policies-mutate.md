<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Mutate*</small>

# Mutate Configurations 

The ```mutate``` rule contains actions that should be applied to the resource before its creation. Mutation can be made using patches or overlay. Using ```patches``` in the JSONPatch format, you can make point changes to the created resource, and ```overlays``` are designed to bring the resource to the desired view according to a specific pattern.

Resource mutation occurs before validation, so the validation rules should not contradict the changes set in the mutation section.


---
<small>*Read Next >> [Generate](/documentation/writing-policies-generate.md)*</small>
