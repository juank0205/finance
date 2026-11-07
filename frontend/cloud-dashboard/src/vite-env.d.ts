interface ImportMetaEnv {
  /** Base URL of cloud-api. Unset = demo data. */
  readonly VITE_API_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
