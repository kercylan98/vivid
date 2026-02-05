declare module 'mermaid' {
  interface RunOptions {
    nodes?: HTMLElement[];
    querySelector?: string;
  }
  interface RenderResult {
    svg: string;
    bindFunctions?: (element: HTMLElement) => void;
  }
  interface MermaidAPI {
    initialize(config: {
      startOnLoad?: boolean;
      theme?: string;
      securityLevel?: string;
      themeVariables?: Record<string, string>;
      themeCSS?: string;
      fontFamily?: string;
    }): void;
    run(options?: RunOptions): Promise<unknown>;
    render(id: string, text: string): Promise<RenderResult>;
  }
  const mermaid: MermaidAPI;
  export default mermaid;
}
