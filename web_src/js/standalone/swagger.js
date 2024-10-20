window.addEventListener('load', async () => {
  const [{default: SwaggerUI}] = await Promise.all([
    import(/* webpackChunkName: "swagger-ui" */'swagger-ui-dist/swagger-ui-es-bundle.js'),
    import(/* webpackChunkName: "swagger-ui" */'swagger-ui-dist/swagger-ui.css'),
  ]);

  const url = document.getElementById('swagger-ui').getAttribute('data-source');
  const res = await fetch(url);
  const spec = await res.json();

  // Make the page's protocol be at the top of the schemes list
  const proto = window.location.protocol.slice(0, -1);
  spec.schemes.sort((a, b) => {
    if (a === proto) return -1;
    if (b === proto) return 1;
    return 0;
  });

  const ui = SwaggerUI({
    spec,
    dom_id: '#swagger-ui',
    deepLinking: true,
    docExpansion: 'none',
    defaultModelRendering: 'model', // don't show examples by default, because they may be incomplete
    presets: [
      SwaggerUI.presets.apis,
    ],
    plugins: [
      SwaggerUI.plugins.DownloadUrl,
    ],
  });

  window.ui = ui;
});
