interface Window {
  config?: {
    appUrl: string;
    appSubUrl: string
  }
}

declare module '*.vue' {
  import Vue from 'vue';
  export default Vue;
}

type CodeMirrorLanguage = typeof import('@codemirror/language');
type CodeMirrorSearch = typeof import('@codemirror/search');
type CodeMirrorState = typeof import('@codemirror/state');
type CodeMirrorView = typeof import('@codemirror/view');
