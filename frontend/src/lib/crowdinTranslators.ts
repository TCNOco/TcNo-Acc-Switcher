/** Mirrors `platform.CrowdinTranslatorsList` from Wails bindings. */
export type CrowdinProofReader = {
  name: string;
  languages: string;
};

export type CrowdinTranslatorsList = {
  proofReaders: CrowdinProofReader[];
  translators: string[];
};
