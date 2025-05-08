# build-okd-images-prow
Builds component images for OKD, taking an existing release as input. It walks through each component image which is part of the release image and triggers a build
for whichever ones are older than the given threshold.

# Requirements
- Need the oc binary to be installed on your system (https://github.com/openshift/oc)
- Need access to RedHat's Openshift app.ci cluster
- Need access to OKD release images (https://amd64.origin.releases.ci.openshift.org/)

# Install and run

You can run this as:
```
go run build-okd-images.go <image-pull-spec> <threshold-days> <optional: release-branch>
```
where
- image-pull-spec: the pull spec of a release image from a particular release here: https://amd64.origin.releases.ci.openshift.org/
- threshold-days: the number of days old an image should be in order to trigger a build
- release-branch: the branch from which to build. This is optional and if not specified, the tool will get it from the manifest metadata of the respective image
