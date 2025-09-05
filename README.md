Baton Keycloak Connector - forked from github.com/spiros-spiros/baton-keycloak

Baton Keycloak Connector is a plugin that integrates Keycloak with Baton, enabling seamless synchronization and provisioning of users and groups.
🔧 Features

    User & Group Synchronization: Fetches users and groups from Keycloak for Baton to manage.

    Provisioning Support: Allows Baton to create, update, and delete users and groups within Keycloak.

    Read-Only Mode: Option to operate in a non-destructive mode, preventing any changes to Keycloak data.

    Customizable Configuration: Supports various Keycloak setups through environment variables or command-line flags.

🚀 Getting Started

Prerequisites

    Go 1.18 or higher

    Access to a running Keycloak instance

    Baton CLI installed

Installation

Clone the repository:

git clone https://github.com/spiros-spiros/baton-keycloak.git

cd baton-keycloak

Build the connector:

go build -o baton-keycloak

Configuration

Set the following environment variables or pass them as command-line flags:

    KEYCLOAK_URL: Base URL of your Keycloak instance (e.g., https://keycloak.example.com)

    KEYCLOAK_REALM: Name of the realm to connect to

    KEYCLOAK_CLIENT_ID: Client ID for authentication

    KEYCLOAK_CLIENT_SECRET: Client secret for authentication

    BATON_CLIENT_ID: Credentials to connect to Baton (will do a one off sync if not supplied)

    BATON_CLIENT_SECRET: Credentials to connect to Baton (will do a one off sync if not supplied)

Usage

Run the connector:

./baton-keycloak --provisioning

This will start the connector, allowing Baton to interact with your Keycloak instance. Omit the --provisioning flag for read-only.

🐳 Docker Support

A Dockerfile is included for containerized deployments.

Build the Docker image:

docker build -t baton-keycloak .

Run the container:

docker run -e KEYCLOAK_URL=https://keycloak.example.com \
           -e KEYCLOAK_REALM=your-realm \
           -e KEYCLOAK_CLIENT_ID=your-client-id \
           -e KEYCLOAK_CLIENT_SECRET=your-client-secret \
           baton-keycloak

📄 License

This project is licensed under the Apache 2.0 License.

🤝 Contributing

Contributions are welcome! Please open issues or submit pull requests for any enhancements or bug fixes.

📫 Contact

For questions or support, please open an issue on the GitHub repository.
