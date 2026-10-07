const {defineConfig}=require('@playwright/test');
module.exports=defineConfig({
 testDir:'./e2e', workers:1, retries:0, timeout:45000,
 use:{baseURL:'http://127.0.0.1:8080', screenshot:'only-on-failure', trace:'retain-on-failure'},
 reporter:[['list'],['html',{open:'never'}]],
 webServer:{command:'go run .',url:'http://127.0.0.1:8080',reuseExistingServer:false,timeout:120000,
 env:{APP_ENV:'test',RENDER:'false',PORT:'8080',DATABASE_URL:'',ADMIN_USERNAME:'browseradmin',ADMIN_PASSWORD:'browser-test-password',SHOW_RESET_LINKS:'1',SECURE_COOKIES:'0'}}
});
