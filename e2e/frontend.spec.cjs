const {test,expect}=require('@playwright/test');
const {default:AxeBuilder}=require('@axe-core/playwright');
const fs=require('node:fs');
const PNG=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFklEQVR4nGOUs9FgYGBgYmBgYGBgAAAGOACGNuhQ0wAAAABJRU5ErkJggg==','base64');
async function checkPage(page,testInfo,name){
 await expect(page.locator('h1').first()).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth <= window.innerWidth+1)).toBeTruthy();
 const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa']).analyze();
 expect(result.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))}))).toEqual([]);
 await page.screenshot({path:testInfo.outputPath(name+'.png'),fullPage:true});
}
test('customer journeys, public sessions and narrow navigation',async({page},testInfo)=>{
 const email='browser-customer-'+Date.now()+'@example.test';
 const response=await page.request.post('/customer/register',{multipart:{fullname:'A customer with a deliberately long display name',phone:'08012345678',email,password:'browser-pass-123',confirm_password:'browser-pass-123'}});
 expect(response.ok(),await response.text()).toBeTruthy();
 for(const width of [1280,320]){
  await page.setViewportSize({width,height:900});
  for(const route of ['/customer/dashboard','/customer/bookings','/customer/profile','/customer/chats','/customer/barbers']){
   await page.goto(route);
   await expect(page.getByRole('navigation',{name:'customer navigation'})).toBeVisible();
   await checkPage(page,testInfo,width+'-'+route.split('/').pop());
  }
 }
 await page.goto('/');
 await expect(page.getByRole('link',{name:'Customer dashboard',exact:true})).toBeVisible();
 await page.goto('/explore');
 await expect(page.getByRole('link',{name:'Dashboard',exact:true})).toBeVisible();
 const response404=await page.goto('/missing-page');
 expect(response404.status()).toBe(404);
 await expect(page.getByRole('navigation',{name:'Recovery options'})).toBeVisible();
});
test('barber and admin navigation and mobile field labels',async({page},testInfo)=>{
 const email='browser-barber-'+Date.now()+'@example.test';
 const response=await page.request.post('/barber/register',{multipart:{fullname:'A barber with a long display name',phone:'08012345678',email,shopname:'A shop with a deliberately long name',shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}});
 expect(response.ok(),await response.text()).toBeTruthy();
 const login=await page.request.post('/barber/login',{form:{email,password:'browser-pass-123'}});
 expect(login.ok(),await login.text()).toBeTruthy();
 await page.setViewportSize({width:320,height:900});
 for(const route of ['/barber/dashboard','/barber/profile','/barber/bookings','/barber/chats']){
  await page.goto(route);await checkPage(page,testInfo,route.split('/').join('-'));
  await expect(page.getByRole('navigation',{name:'barber navigation'}).getByRole('link',{name:'Schedule',exact:true})).toBeVisible();
 }
 const admin=await page.request.post('/admin/login',{form:{username:'browseradmin',password:'browser-test-password'}});
 expect(admin.ok(),await admin.text()).toBeTruthy();
 await page.goto('/admin/barbers');await checkPage(page,testInfo,'admin-barbers');
 const label=page.locator('td[data-label="Barber"]').first();
 await expect(label).toBeVisible();
 expect(await label.evaluate(el=>getComputedStyle(el,'::before').content)).toBe('"Barber"');
});
test('native viewer keeps profile links and service selection keyboard accessible',async({page})=>{
 await page.goto('/');
 await page.setContent('<a id="profile" href="/explore">Profile<img data-fullscreen-image src="data:image/png;base64,'+PNG.toString('base64')+'" alt="Profile"></a><input id="style" type="radio"><label for="style"><img data-fullscreen-image src="data:image/png;base64,'+PNG.toString('base64')+'" alt="Haircut"></label>');
 await page.addScriptTag({content:fs.readFileSync('static/image-viewer.js','utf8')});
 await page.evaluate(()=>document.dispatchEvent(new Event('DOMContentLoaded')));
 await page.locator('label img').click();await expect(page.locator('#style')).toBeChecked();
 const trigger=page.getByRole('button',{name:'Enlarge Haircut'});
 await trigger.focus();await page.keyboard.press('Enter');
 const dialog=page.getByRole('dialog',{name:'Full-size image'});await expect(dialog).toBeVisible();
 const close=page.getByRole('button',{name:'Close image'});await expect(close).toBeFocused();
 await page.keyboard.press('Tab');await expect(close).toBeFocused();
 await page.evaluate(()=>document.querySelector('#profile').focus());await expect(close).toBeFocused();
 await page.keyboard.press('Escape');await expect(dialog).not.toBeVisible();await expect(trigger).toBeFocused();
 await page.locator('#profile').click();await expect(page).toHaveURL(/\/explore$/);
});
