interface Window {
  __webpack_public_path__: string;
  config?: {
    appUrl: string;
    appSubUrl: string;
    assetUrlPrefix: string;
    pageData: PageData;
    i18n: Record<string, string>;
    customEmojis: Set<string>;
    mentionValues: MentionValue[];
  }
}

interface PageData {
  PLURALSTRINGS_LANG: Record<string, string[]>;
  PLURAL_RULE_LANG: number;
  DATETIMESTRINGS: Record<string, string>;

  [key: string]: unknown; // TODO: remove this after we've enumerated all pageData entries
}

interface MentionValue {
  key: string;
  value: string;
  name: string;
  fullname: string;
  avatar: string;
}

declare module '*.vue' {
  import Vue from 'vue';
  export default Vue;
}

type CodeMirrorLanguage = typeof import('@codemirror/language');
type CodeMirrorSearch = typeof import('@codemirror/search');
type CodeMirrorState = typeof import('@codemirror/state');
type CodeMirrorView = typeof import('@codemirror/view');
