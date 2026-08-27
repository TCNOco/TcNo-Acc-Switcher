export type CrowdinProofReader = {
  name: string;
  languages: string;
};

/** Mirrors `platform.CrowdinTranslatorsList` from Wails bindings. */
export type CrowdinTranslatorsList = {
  proofReaders: CrowdinProofReader[];
  translators: string[];
};
