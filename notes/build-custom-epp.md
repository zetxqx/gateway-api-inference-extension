# Building and Pushing a Custom EPP Image

This guide provides step-by-step instructions for building a custom Endpoint Picker Protocol (EPP) image from your local source code and pushing it to your private Google Artifact Registry.

## Prerequisites

*   You have `docker` installed and configured.
*   You have `gcloud` CLI installed and authenticated.
*   You have a Google Cloud Project.
*   You have cloned the `gateway-api-inference-extension` repository.

## Steps

### 1. Set Up Environment Variables

First, set the following environment variables in your shell. We will use the current Git commit SHA as the image tag, which is a best practice for ensuring your image is traceable and your tests are reproducible.

```bash
# Replace <your_project_id> with your actual Google Cloud Project ID
export PROJECT_ID=<your_project_id>

# Set the Artifact Registry location (e.g., us-central1)
export REGION=us-central1

# The name of the repository in Artifact Registry
export AR_REPO_NAME="gateway-api-inference-extension"

# Define the image repository and tag for your custom EPP image
export IMAGE_REPO="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO_NAME}/epp"
export IMAGE_TAG=$(git rev-parse HEAD)
```

### 2. Create the Artifact Registry Repository

This command creates a new Docker repository in your Google Cloud project's Artifact Registry. If the repository already exists, the command will do nothing.

```bash
gcloud artifacts repositories create "${AR_REPO_NAME}" \
    --repository-format=docker \
    --location="${REGION}" \
    --description="Gateway API Inference Extension images" \
    --project="${PROJECT_ID}"
```

### 3. Configure Docker Authentication

Configure Docker to use your `gcloud` credentials to authenticate with Artifact Registry.

```bash
gcloud auth configure-docker "${REGION}-docker.pkg.dev"
```

### 4. Build the Custom EPP Image

From the root of the `gateway-api-inference-extension` repository, run the following `make` command to build your custom EPP image. This command overrides the default image repository and tag with the ones you defined.

The `make image-build` command uses `docker buildx` to build the image.

```bash
make image-build IMAGE_REPO=${IMAGE_REPO} IMAGE_TAG=${IMAGE_REPO}:${IMAGE_TAG}
```

If you want to load the image into your local docker daemon, you can use `image-local-load`.

```bash
make image-local-load IMAGE_REPO=${IMAGE_REPO} IMAGE_TAG=${IMAGE_REPO}:${IMAGE_TAG}
```

After the build completes, you can verify that the image exists in your local Docker registry by running:

```bash
docker images "${IMAGE_REPO}"
```

### 5. Push the Image to Artifact Registry

After the build is successful, push the image to your private Artifact Registry.

```bash
make image-push IMAGE_REPO=${IMAGE_REPO} IMAGE_TAG=${IMAGE_REPO}:${IMAGE_TAG}
```

You can verify that the image was pushed by navigating to **Artifact Registry** in your Google Cloud Console or by running:

```bash
gcloud container images describe "${IMAGE_REPO}:${IMAGE_TAG}"
```

You should see details about your image, confirming it was pushed successfully.


### 6. Using Your Custom Image

To use your custom image in a deployment, you will need to update the image reference in your Kubernetes manifests or Helm chart values.

For example, if you are using the Helm chart, you would update the `values.yaml` file or use the `--set` flag during installation:

```yaml
# in your values.yaml
image:
  repository: us-central1-docker.pkg.dev/your_project_id/gateway-api-inference-extension/epp
  tag: <your_git_commit_sha>
```

Or with `helm install`:

```bash
helm install my-release ./config/charts/inferencepool \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG}
```

## Best Practices for Image Tagging

### Why Avoid the `latest` Tag?

Using the `latest` tag is convenient for local development but is highly discouraged for testing and production environments. Here's why:

*   **It's Mutable**: The `latest` tag is a "floating" tag that can point to different images over time. When you push a new image with the `latest` tag, the old one is overwritten.
*   **Difficult to Track**: If you encounter a bug, it's hard to know which version of the code is actually running if it's just tagged as `latest`.
*   **Unreliable Deployments**: Kubernetes' default `imagePullPolicy` is `IfNotPresent`. If a node already has an image with the `latest` tag, it won't pull the new version, leading to inconsistent deployments where different nodes run different code.

### Use Immutable Tags

The best practice is to use **immutable tags** that are unique and never change. This ensures that you always know the exact version of the code running in your cluster. The most reliable and traceable method is to use the **Git commit SHA** as your image tag, as we've done in this guide. This provides a direct link between your container image and the source code it was built from.
