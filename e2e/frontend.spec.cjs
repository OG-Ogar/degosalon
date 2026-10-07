const {test,expect}=require('@playwright/test');
const {default:AxeBuilder}=require('@axe-core/playwright');
const fs=require('node:fs');
const PNG=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFklEQVR4nGOUs9FgYGBgYmBgYGBgAAAGOACGNuhQ0wAAAABJRU5ErkJggg==','base64');
test('both conversation views deliver replies through explicit refresh',async({browser},testInfo)=>{
 const customerContext=await browser.newContext();
 const barberContext=await browser.newContext();
 try {
  const customer=await customerContext.newPage();
  const barber=await barberContext.newPage();
  const suffix=Date.now();
  const email='chat-barber-'+suffix+'@example.test';
  const registered=await barber.request.post('/barber/register',{multipart:{fullname:'Chat Barber',phone:'08012345678',email,shopname:'Chat Shop',shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}});
  expect(registered.ok(),await registered.text()).toBeTruthy();
  expect((await barber.request.post('/barber/login',{form:{email,password:'browser-pass-123'}})).ok()).toBeTruthy();
  const signedUp=await customer.request.post('/customer/register',{multipart:{fullname:'Chat Customer',phone:'08012345678',email:'chat-customer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}});
  expect(signedUp.ok(),await signedUp.text()).toBeTruthy();
  await customer.goto('/customer/barbers?state=Lagos');
  await customer.getByRole('link',{name:/Chat Shop/}).first().click();
  await customer.getByRole('link',{name:'Message Barber',exact:true}).first().click();
  await expect(customer.getByRole('link',{name:'Refresh conversation',exact:true})).toBeVisible();
  await customer.getByRole('textbox',{name:'Your message'}).fill('Customer greeting');
  await customer.getByRole('button',{name:/send/i}).click();
  await barber.goto('/barber/chats');
  await barber.getByRole('link').filter({hasText:'Chat Customer'}).first().click();
  await expect(barber.getByText('Customer greeting',{exact:true})).toBeVisible();
  await barber.getByRole('textbox',{name:'Your message'}).fill('Barber reply');
  await barber.getByRole('button',{name:/send/i}).click();
  await expect(customer.getByText('Barber reply',{exact:true})).toHaveCount(0);
  await customer.getByRole('link',{name:'Refresh conversation',exact:true}).click();
  await expect(customer.getByText('Barber reply',{exact:true})).toBeVisible();
  for(const width of [1280,320]){
   for(const [page,role] of [[customer,'customer'],[barber,'barber']]){
    await page.setViewportSize({width,height:900});
    await checkPage(page,testInfo,role+'-conversation-'+width);
    const box=page.getByRole('textbox',{name:'Your message'});
    await expect(box).toHaveAttribute('aria-describedby',/\S+/);
   }
  }
 } finally {
  await customerContext.close();
  await barberContext.close();
 }
});
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

test('canonical geography depends on state and preserves native no-script discovery',async({page,browser},testInfo)=>{
 await page.goto('/explore');
 await expect(page.locator('#state option')).toHaveCount(38);
 await page.locator('#state').selectOption({label:'Lagos'});
 await expect(page.locator('#area option')).toHaveCount(21);
 await page.locator('#area').selectOption({label:'Ikeja'});
 await page.locator('#state').selectOption({label:'FCT'});
 await expect(page.locator('#area option')).toHaveCount(7);
 await expect(page.locator('#area')).toHaveValue('');
 await page.locator('#area').selectOption({label:'Abuja Municipal'});
 await page.getByRole('button',{name:/Find barbers/i}).click();
 await expect(page.locator('#state')).toHaveValue('NG015');
 await expect(page.locator('#area')).toHaveValue('NG015002');
 await page.setViewportSize({width:320,height:900});
 await checkPage(page,testInfo,'geography-fct-mobile');
 await page.goto('/barber/register');
 await page.locator('#state').selectOption({label:'Lagos'});
 await page.locator('#local_area').selectOption({label:'Ikeja'});
 await expect(page.locator('#local_area')).toHaveAttribute('aria-describedby','geography-help');
 const context=await browser.newContext({javaScriptEnabled:false});
 try {
  const native=await context.newPage();await native.goto('/explore');
  await expect(native.locator('#area option')).toHaveCount(775);
  await native.locator('#state').selectOption({label:'Lagos'});
  await native.locator('#area').selectOption({label:'Ikeja'});
  await native.getByRole('button',{name:/Find barbers/i}).click();
  await expect(native.locator('#state')).toHaveValue('NG025');
  await expect(native.locator('#area')).toHaveValue('NG025011');
 } finally {await context.close();}
});

test('help and policy placeholders are reachable and accessible on mobile',async({page},testInfo)=>{
 await page.setViewportSize({width:320,height:900});
 await page.goto('/');
 const footer=page.getByRole('navigation',{name:'Help and policies'});
 await expect(footer.getByRole('link',{name:'Support',exact:true})).toHaveAttribute('href','/support');
 for(const route of ['/help','/support','/privacy','/terms']){
  const response=await page.goto(route);expect(response.status()).toBe(200);
  expect(response.headers()['x-robots-tag']).toBe('noindex, follow');
  await expect(page.getByText('Placeholder — awaiting approved content.',{exact:true})).toBeVisible();
  await checkPage(page,testInfo,'content-'+route.slice(1));
  await expect(page.locator('meta[property="og:title"]')).toHaveCount(1);
 }
});

test('booking details reflect cancellation after the original request',async({browser},testInfo)=>{
 const barberContext=await browser.newContext();const customerContext=await browser.newContext();
 try {
  const barber=await barberContext.newPage();const customer=await customerContext.newPage();const suffix=Date.now();
  const email='status-barber-'+suffix+'@example.test';
  const created=await barber.request.post('/barber/register',{multipart:{fullname:'Status Barber',phone:'08012345678',email,shopname:'Status Journey Shop',shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}});expect(created.ok(),await created.text()).toBeTruthy();
  expect((await barber.request.post('/barber/login',{form:{email,password:'browser-pass-123'}})).ok()).toBeTruthy();
  await barber.goto('/barber/hairstyles/new');
  for(const [name,value] of Object.entries({name:'Status Journey Cut',category:'Fade',description:'A simple cut',price:'1000',duration:'30'})){await barber.locator('[name="'+name+'"]').fill(value);}
  await barber.locator('[name="available"]').check();await barber.getByRole('button',{name:'Save Hairstyle',exact:true}).click();
  const registered=await customer.request.post('/customer/register',{multipart:{fullname:'Status Customer',phone:'08012345678',email:'status-customer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}});expect(registered.ok(),await registered.text()).toBeTruthy();
  await customer.goto('/customer/barbers?state=Lagos&area=Ikeja');await customer.getByRole('link',{name:'Status Journey Shop',exact:true}).first().click();
  const date=new Date(Date.now()+2*24*60*60*1000).toISOString().slice(0,10);
  await customer.locator('#availability-date').fill(date);await customer.getByRole('button',{name:'Show available times',exact:true}).click();
  const time=await customer.locator('#time option').nth(1).getAttribute('value');expect(time).toBeTruthy();await customer.locator('#time').selectOption(time);
  await customer.getByRole('button',{name:/Request Appointment/}).click();await expect(customer).toHaveURL(/\/customer\/booking-confirmation\/\d+/);
  const confirmation=customer.url();await expect(customer.locator('.domain-status').filter({hasText:'Pending'})).toHaveCount(2);
  await customer.goto('/customer/bookings');customer.once('dialog',dialog=>dialog.accept());await customer.locator('form[action$="/cancel"] button').first().click();
  await customer.goto(confirmation);await expect(customer.locator('.domain-status').filter({hasText:'Cancelled'})).toHaveCount(2);
  await expect(customer.getByText('This appointment has been cancelled.',{exact:true})).toBeVisible();
  await expect(customer.getByText('Pending Barber Approval',{exact:true})).toHaveCount(0);
  await customer.setViewportSize({width:320,height:900});await checkPage(customer,testInfo,'cancelled-booking-details');
 } finally {await barberContext.close();await customerContext.close();}
});

test('shared forms and archive confirmations work at phone width',async({page},testInfo)=>{
 const suffix=Date.now();const email='primitives-'+suffix+'@example.test';
 expect((await page.request.post('/barber/register',{multipart:{fullname:'Primitive Barber',phone:'08012345678',email,shopname:'Primitive Shop '+suffix,shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
 expect((await page.request.post('/barber/login',{form:{email,password:'browser-pass-123'}})).ok()).toBeTruthy();
 await page.setViewportSize({width:320,height:900});
 for(const route of ['/barber/products/new','/barber/hairstyles/new','/barber/schedule','/barber/profile/edit']){
  await page.goto(route);await checkPage(page,testInfo,'primitives-'+route.replaceAll('/','-'));
  const save=page.locator('main button[type=submit]').first();
  expect((await save.boundingBox()).height).toBeGreaterThanOrEqual(44);
 }
 await page.goto('/barber/hairstyles/new');
 for(const [name,value] of Object.entries({name:'Archive journey cut',category:'Fade',description:'A simple cut',price:'1000',duration:'30'}))await page.locator('[name="'+name+'"]').fill(value);
 await page.locator('[name="available"]').check();await page.getByRole('button',{name:'Save Hairstyle',exact:true}).click();
 await page.locator('a[href$="/delete"]').filter({hasText:/archive/i}).first().click();
 await checkPage(page,testInfo,'primitives-archive-hairstyle');
 await expect(page.locator('.ui-actions input[name=csrf_token]')).toHaveValue(/\S+/);
 await page.getByRole('link',{name:'No, keep it',exact:true}).click();
 await expect(page.getByText('Archive journey cut',{exact:true}).first()).toBeVisible();
 expect((await page.request.post('/admin/login',{form:{username:'browseradmin',password:'browser-test-password'}})).ok()).toBeTruthy();
 await page.goto('/admin/barbers/new');await checkPage(page,testInfo,'primitives-admin-form');
 await page.goto('/admin/barbers');await page.getByRole('link',{name:'Primitive Barber',exact:true}).click();
 await page.locator('a[href$="/delete"]').click();await checkPage(page,testInfo,'primitives-archive-account');
 await page.getByRole('button',{name:'Yes, archive this barber',exact:true}).click();
 await expect(page.getByText('Archived',{exact:true}).first()).toBeVisible();
});
