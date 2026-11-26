/**
 * Copyright (c) 2006-2024, JGraph Holdings Ltd
 * Copyright (c) 2006-2024, draw.io AG
 */
// draw.war's PreConfig.js with offline defaults added by svalbard. Without
// them the editor loads Google Drive's client from apis.google.com on start.
// offline=1 drops the cloud storage backends; stealth=1 disables every
// feature that calls an external web service.
// Overrides of global vars need to be pre-loaded
window.DRAWIO_PUBLIC_BUILD = true;
window.EXPORT_URL = null; // With null, export to PDF uses the print dialog
window.DRAWIO_BASE_URL = null;
window.DRAWIO_VIEWER_URL = null;
window.DRAWIO_LIGHTBOX_URL = null;
window.DRAW_MATH_URL = 'math4/es5';
window.DRAWIO_CONFIG = null;
urlParams['sync'] = 'manual';
urlParams['offline'] = '1';
urlParams['stealth'] = '1';
