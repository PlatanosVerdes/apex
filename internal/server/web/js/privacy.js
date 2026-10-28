import { lang } from './i18n.js';

for (const article of document.querySelectorAll('article')) article.hidden = article.lang !== lang;
document.documentElement.lang = lang;
if (lang === 'en') document.title = 'Apex | Privacy';
