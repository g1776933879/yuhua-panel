import { createApp } from 'vue';
import 'ant-design-vue/dist/reset.css';
import Home from './Home.vue';
import './styles.css';
import { bootWatchdog } from './watchdog';

console.log('欢迎使用青玄面板！');
console.log('GITHUB开源地址 https://github.com/smallfawn/sillyGirl');
bootWatchdog('home');
createApp(Home).mount('#home-root');
