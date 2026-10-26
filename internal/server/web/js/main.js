import { Api } from './api.js';
import { ApexApp, applyLegColours } from './components/apex-app.js';

applyLegColours();
document.body.append(new ApexApp(new Api()));
