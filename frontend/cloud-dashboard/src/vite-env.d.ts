interface ImportMetaEnv {
  /** Base URL of cloud-api. Unset = demo data. */
  readonly VITE_API_URL?: string
  /** Cognito hosted sign-in page, https://<prefix>.auth.<region>.amazoncognito.com. Unset = no sign-in. */
  readonly VITE_COGNITO_DOMAIN?: string
  /** The dashboard's Cognito app client id. */
  readonly VITE_COGNITO_CLIENT_ID?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
