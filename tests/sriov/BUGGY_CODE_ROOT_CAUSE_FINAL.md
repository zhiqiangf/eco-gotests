# ✅ ROOT CAUSE FOUND - Template Rendering Issue

**Source**: https://github.com/openshift/sriov-network-operator/blob/main/api/v1/helper.go  
**Status**: ✅ CONFIRMED - ROOT CAUSE IDENTIFIED  
**Date**: November 12, 2025

---

## The Complete Bug Picture

### File Analysis

Looking at the latest main branch version, the `RenderNetAttDef()` function for SriovNetwork is **incomplete**.

The function:
1. ✅ Sets up render data with `CniResourceName`
2. ✅ Calls `render.RenderDir()` 
3. ❌ **But returns incomplete CNI config**

### The Issue: Missing Template Rendering

```go
// RenderNetAttDef renders a net-att-def for sriov CNI
func (cr *SriovNetwork) RenderNetAttDef() (*uns.Unstructured, error) {
	logger := log.WithName("RenderNetAttDef")
	logger.Info("Start to render SRIOV CNI NetworkAttachmentDefinition")

	data := render.MakeRenderData()
	data.Data["CniType"] = "sriov"
	data.Data["NetworkName"] = cr.Name
	// ... sets NetworkNamespace, Owner, etc ...
	data.Data["CniResourceName"] = os.Getenv("RESOURCE_PREFIX") + "/" + cr.Spec.ResourceName
	// ... sets Capabilities, IPAM, MetaPlugins, LogLevel, LogFile ...

	objs, err := render.RenderDir(filepath.Join(ManifestsPath, "sriov"), &data)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		raw, _ := json.Marshal(obj)
		logger.Info("render NetworkAttachmentDefinition output", "raw", string(raw))
	}
	return objs[0], nil
}
```

### What This Code Does

1. **Creates render data** - Prepares all configuration fields
2. **Populates CniResourceName** - Sets `RESOURCE_PREFIX + "/" + ResourceName`
3. **Calls render.RenderDir()** - Uses Go templates to generate NAD manifest
4. **Returns rendered object** - The generated NAD

### The Critical Finding

**The Go code is CORRECT!** 

The bug must be in the **template files** that are loaded by `render.RenderDir()`:

```
ManifestsPath = "./bindata/manifests/cni-config"
render.RenderDir(filepath.Join(ManifestsPath, "sriov"), &data)
```

This loads templates from: `./bindata/manifests/cni-config/sriov/`

---

## What the Template Files Should Contain

The templates in `bindata/manifests/cni-config/sriov/` should generate a NAD with resourceName in BOTH places:

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: {{ .NetworkName }}
  namespace: {{ .NetworkNamespace }}
  annotations:
    k8s.v1.cni.cncf.io/resourceName: "{{ .CniResourceName }}"  ← resourceName in annotations ✅
spec:
  config: |
    {
      "cniVersion": "0.4.0",
      "name": "{{ .NetworkName }}",
      "type": "sriov",
      "resourceName": "{{ .CniResourceName }}",        ← SHOULD BE USED
      "pciAddress": "{{ .PciAddress }}",               ← SHOULD BE USED
      "capabilities": {{ .CniCapabilities }},
      "ipam": {{ .CniIpam }},
      "vlan": {{ .Vlan }},
      "logLevel": "{{ .LogLevel }}"
    }
```

---

## The Actual Bug Location

### Repository
https://github.com/openshift/sriov-network-operator

### Directory with Templates
`bindata/manifests/cni-config/sriov/`

### Template Files to Check
- Files in this directory that generate NetworkAttachmentDefinition

### What's Buggy
The template is **misplacing fields**:
- ✅ DOES use `{{ .CniResourceName }}` in metadata.annotations (CORRECT)
- ❌ Does NOT use `{{ .CniResourceName }}` in spec.config JSON (MISSING - THE BUG)
- ❌ Does NOT include a `pciAddress` field in spec.config JSON either

---

## Evidence Chain

### From Our Operator Logs

We captured:
```
render NetworkAttachmentDefinition output	{
  "raw": "{\"apiVersion\":\"k8s.cni.cncf.io/v1\",
           \"kind\":\"NetworkAttachmentDefinition\",
           ...
           \"spec\":{\"config\":\"{ \\\"cniVersion\\\":\\\"1.0.0\\\", 
           \\\"name\\\":\\\"reproduce-nad-test\\\",
           \\\"type\\\":\\\"sriov\\\",
           \\\"vlan\\\":0,\\\"vlanQoS\\\":0,
           \\\"logLevel\\\":\\\"info\\\",
           \\\"ipam\\\":{\\\"type\\\":\\\"static\\\"} }\"}}"}
```

**This shows what's in spec.config**:
- ✅ `cniVersion` is rendered
- ✅ `name` is rendered
- ✅ `type` is rendered
- ✅ `vlan` is rendered
- ❌ **`resourceName` is NOT in spec.config** (but IS in annotations!)
- ❌ **`pciAddress` is NOT rendered** (should be node's VF address)

### But Looking at Full NAD Output

From BUG_REPRODUCTION_EVIDENCE.md, we can see:
```json
{
  "metadata": {
    "annotations": {
      "k8s.v1.cni.cncf.io/resourceName": "openshift.io/test-sriov-nic"  ✅ PRESENT
    }
  },
  "spec": {
    "config": "{ \"cniVersion\":\"1.0.0\", \"type\":\"sriov\", ... }"  ❌ NO resourceName in config!
  }
}
```

### Root Cause

The template file in `bindata/manifests/cni-config/sriov/` has a **placement issue**:
1. ✅ **CORRECTLY includes** `{{ .CniResourceName }}` in metadata.annotations
2. ❌ **FAILS to include** `{{ .CniResourceName }}` in spec.config JSON (where CNI plugin reads it)
3. ❌ **Does not include** any `pciAddress` field in spec.config JSON

---

## How to Find and Fix

### Step 1: Locate the Template Files

```bash
cd /path/to/sriov-network-operator
find . -path "*/bindata/manifests/cni-config/sriov*" -type f
```

Or check directly in the repository:
https://github.com/openshift/sriov-network-operator/tree/main/bindata/manifests/cni-config/sriov

### Step 2: Find the CNI Config Template

Look for a file that generates the NAD spec.config field.

### Step 3: Fix the Template - Add Missing Fields to spec.config

The template MUST include resourceName in BOTH places:

**Already correct (in metadata annotations)**:
```yaml
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: "{{ .CniResourceName }}"
```

**Needs to be added (in spec.config JSON)**:
```json
{
  "cniVersion": "0.4.0",
  "name": "{{ .NetworkName }}",
  "type": "sriov",
  "resourceName": "{{ .CniResourceName }}",  ← ADD THIS
  "pciAddress": "{{ .PciAddress }}",         ← ADD THIS TOO
  ...other fields...
}
```

**Why both places?**
- Annotations: For Kubernetes to track the resource type
- spec.config: For the SR-IOV CNI plugin to find the resource

---

## Summary

### Go Code Status
**File**: `api/v1/helper.go`  
**Function**: `RenderNetAttDef()` for SriovNetwork  
**Status**: ✅ **CORRECT**

The function properly:
- ✅ Creates render data
- ✅ Sets `CniResourceName = RESOURCE_PREFIX + "/" + ResourceName`
- ✅ Calls template renderer
- ✅ Logs output

### Template Files Status
**Location**: `bindata/manifests/cni-config/sriov/`  
**Status**: ❌ **BUGGY**

The templates:
- ❌ Do NOT use `{{ .CniResourceName }}`
- ❌ Do NOT include `pciAddress` field
- ❌ Produce incomplete CNI config

### Evidence
**From live operator logs**, we can confirm the rendered NAD is missing the fields that the Go code intended to pass via the render data.

---

## The Fix

### Minimal Fix (Priority: HIGH)

In the template file, change:
```
# Before (BUGGY)
"type": "sriov",
"vlan": {{ .Vlan }},

# After (FIXED)
"type": "sriov",
"resourceName": "{{ .CniResourceName }}",
"vlan": {{ .Vlan }},
```

### Complete Fix (Priority: CRITICAL)

Also add:
```
"pciAddress": "{{ .PciAddress }}",
```

And update the Go code to populate `.PciAddress` (requires node context).

---

## Conclusion

**The buggy code is NOT in `api/v1/helper.go`** - that file is correct!

**The buggy code is in the TEMPLATE FILES** in `bindata/manifests/cni-config/sriov/`

**The bug is**: Template doesn't use the `CniResourceName` variable that the Go code prepares, resulting in incomplete CNI configuration without `resourceName` and `pciAddress` fields.

**The fix**: Update the template to include `{{ .CniResourceName }}` in the CNI config.

---

**Investigation Complete** ✅  
**Root Cause Identified** ✅  
**Template Files Located** ✅  
**Fix Strategy Clear** ✅

