import { Api } from './api.js';
import { lang } from './i18n.js';
import { ApexApp, applyLegColours } from './components/apex-app.js';

applyLegColours();
document.documentElement.lang = lang;
document.body.append(new ApexApp(new Api()));
