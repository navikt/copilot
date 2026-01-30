# My Copilot

My Copilot is a self-service tool for managing your GitHub Copilot subscription. It allows users to activate or deactivate their Copilot subscription and view their subscription details.

## What It Does

- **Subscription Management**: Users can activate or deactivate their GitHub Copilot subscription.
- **Subscription Details**: Users can view details about their current subscription, including plan type, status, last activity, and more.
- **User Information**: Displays user information such as name, email, and groups.

## Integrations

- **GitHub API**: Interacts with the GitHub API to manage Copilot subscriptions and retrieve user details.
  - Uses GitHub Copilot Metrics API (`/orgs/{org}/copilot/metrics`) for usage analytics
  - Uses GitHub Copilot User Management API for seat assignments and billing
  - All API requests include `X-GitHub-Api-Version: 2022-11-28` header for stability
- **Azure AD**: Uses Azure AD for authentication and authorization, ensuring that only authorized users can access the application.
- **Next.js**: Built with Next.js for server-side rendering and optimized performance.
- **Tailwind CSS**: Utilizes Tailwind CSS for styling the application.

## Development

### Prerequisites

- Node.js (version 22 or higher)
- pnpm (version 7 or higher)
- A GitHub App with the necessary permissions
- Azure AD application for authentication

### Getting Started

First, clone the repository:

```bash
git clone https://github.com/nais/my-copilot.git
cd my-copilot
```

Install the dependencies:

```bash
pnpm install
```

Create a `.env.local` file in the root directory and add the required environment variables:

```env
GITHUB_APP_ID=your_github_app_id
GITHUB_APP_PRIVATE_KEY=your_github_app_private_key
GITHUB_APP_INSTALLATION_ID=your_github_app_installation_id
AZURE_APP_CLIENT_ID=your_azure_app_client_id
AZURE_OPENID_CONFIG_JWKS_URI=your_azure_openid_config_jwks_uri
AZURE_OPENID_CONFIG_ISSUER=your_azure_openid_config_issuer
```

Run the development server:

```bash
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

### Building and Testing

To build the project:

```bash
pnpm build
```

To run the tests:

```bash
pnpm test
```

### Deployment

This project uses GitHub Actions for CI/CD. The workflow is defined in `.github/workflows/build-deploy.yaml`. The application is deployed to the Nais platform.

### Group Access

This project uses group access to control who can use GitHub Copilot. The groups are defined in the `app.yaml` file under the `azure.application.claims.groups` section. To give more groups access, you need to add their IDs to this section.

Example:

```yaml
azure:
  application:
    enabled: true
    tenant: nav.no
    allowAllUsers: true
    claims:
      groups:
        - id: 48120347-8582-4329-8673-7beb3ed6ca06
        - id: 76e9ee7e-2cd1-4814-b199-6c0be007d7b4
        - id: eb5c5556-6c9a-4e54-83fc-f70cae25358d
        # Add more group IDs here
```

## Learn More

To learn more about the technologies used in this project, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [GitHub API Documentation](https://docs.github.com/en/rest) - learn about the GitHub API.
- [Azure AD Documentation](https://docs.microsoft.com/en-us/azure/active-directory/) - learn about Azure AD.
- [Tailwind CSS Documentation](https://tailwindcss.com/docs) - learn about Tailwind CSS.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request with your changes.

## License

This project is licensed under the MIT License.
