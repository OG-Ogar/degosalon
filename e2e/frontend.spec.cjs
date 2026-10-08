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
 await trigger.click();await dialog.locator('img').click();await expect(dialog).toBeVisible();
 await close.click();await expect(dialog).not.toBeVisible();await expect(trigger).toBeFocused();
 await trigger.click();await page.mouse.click(1,1);await expect(dialog).not.toBeVisible();await expect(trigger).toBeFocused();
 await expect(page.locator('body')).not.toHaveClass(/image-viewer-open/);
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

test('approved support contacts and policy placeholders are accessible on mobile',async({page},testInfo)=>{
 await page.setViewportSize({width:320,height:900});
 await page.goto('/');
 const footer=page.getByRole('navigation',{name:'Help and policies'});
 await expect(footer.getByRole('link',{name:'Support',exact:true})).toHaveAttribute('href','/support');
 for(const route of ['/help','/privacy','/terms']){
  const response=await page.goto(route);expect(response.status()).toBe(200);
  expect(response.headers()['x-robots-tag']).toBe('noindex, follow');
  await expect(page.getByText('Placeholder — awaiting approved content.',{exact:true})).toBeVisible();
  await checkPage(page,testInfo,'content-'+route.slice(1));
  await expect(page.locator('meta[property="og:title"]')).toHaveCount(1);
 }
 const support=await page.goto('/support');expect(support.status()).toBe(200);expect(support.headers()['x-robots-tag']).toBeUndefined();
 await expect(page.getByRole('link',{name:'08128162561',exact:true})).toHaveAttribute('href','tel:+2348128162561');
 await expect(page.getByRole('link',{name:'09164226427',exact:true})).toHaveAttribute('href','tel:+2349164226427');
 await expect(page.getByText('Placeholder — awaiting approved content.',{exact:true})).toHaveCount(0);
 await expect(page.locator('link[rel=canonical]')).toHaveAttribute('href','https://example.test/support');
 await checkPage(page,testInfo,'support-approved-contacts');
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
  await barber.locator('[name="image"]').setInputFiles({name:'cut.png',mimeType:'image/png',buffer:PNG});
  await barber.locator('[name="available"]').check();await barber.getByRole('button',{name:'Save Hairstyle',exact:true}).click();
  const registered=await customer.request.post('/customer/register',{multipart:{fullname:'Status Customer',phone:'08012345678',email:'status-customer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}});expect(registered.ok(),await registered.text()).toBeTruthy();
  await customer.goto('/customer/barbers?state=Lagos&area=Ikeja');await customer.getByRole('link',{name:'Status Journey Shop',exact:true}).first().click();
  await expect(customer.locator('[data-booking-summary]')).toContainText('Status Journey Cut · ₦1,000.00 · 30 minutes');
  const service=customer.locator('input[name=hairstyle_id]');
  await service.evaluate(input=>{input.checked=false;});
  await customer.locator('label .style-image img').click();await expect(service).toBeChecked();
  await expect(customer.getByRole('dialog')).toBeHidden();
  const enlarge=customer.getByRole('button',{name:'Enlarge Photo of Status Journey Cut',exact:true});
  await enlarge.click();await expect(customer.getByRole('dialog')).toBeVisible();await expect(service).toBeChecked();
  await customer.keyboard.press('Escape');await expect(enlarge).toBeFocused();
  const nativeContext=await browser.newContext({javaScriptEnabled:false});
  try {
   const native=await nativeContext.newPage();
   expect((await native.request.post('/customer/login',{form:{email:'status-customer-'+suffix+'@example.test',password:'browser-pass-123'}})).ok()).toBeTruthy();
   await native.goto(customer.url());
   await expect(native.locator('[data-booking-summary]')).toContainText('Status Journey Cut · ₦1,000.00 · 30 minutes');
   await expect(native.locator('.image-viewer-trigger')).toHaveCount(0);
   await native.locator('input[name=hairstyle_id]').evaluate(input=>{input.checked=false;});
   await native.locator('label .style-image img').click();await expect(native.locator('input[name=hairstyle_id]')).toBeChecked();
  } finally {await nativeContext.close();}
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

test('server errors and field help remain associated with and without JavaScript',async({browser})=>{
 for(const javaScriptEnabled of [true,false]){
  const context=await browser.newContext({javaScriptEnabled});
  try {
   const page=await context.newPage();await page.goto('/customer/register');
   await page.locator('#fullname').fill('Feedback Customer');await page.locator('#phone').fill('abc');
   await page.locator('#email').fill('feedback-'+Date.now()+'@example.test');
   await page.locator('#password').fill('browser-pass-123');await page.locator('#confirm_password').fill('browser-pass-123');
   await page.locator('main button[type=submit]').click();
   await expect(page.locator('#form-error')).toHaveAttribute('role','alert');
   await expect(page.locator('form[action="/customer/register"]')).toHaveAttribute('aria-describedby','form-error');
   await expect(page.locator('#fullname')).toHaveValue('Feedback Customer');
   const help=(await page.locator('#profile_image').getAttribute('aria-describedby')).split(' ');
   for(const id of help)await expect(page.locator('[id="'+id+'"]')).toHaveCount(1);
   await page.goto('/admin/login');await page.locator('#username').fill('browseradmin');await page.locator('#password').fill('incorrect-password');await page.locator('main button[type=submit]').click();
   await expect(page.locator('form[action="/admin/login"]')).toHaveAttribute('aria-describedby','form-error');
   await expect(page.locator('#form-error')).toBeVisible();
  } finally {await context.close();}
 }
});

test('native select feedback clears on selection and preserves field help',async({page})=>{
 await page.goto('/barber/register');await page.locator('main button[type=submit]').click();
 await expect(page.locator('#local_area')).toHaveAttribute('aria-invalid','true');
 await page.locator('#state').selectOption({label:'Lagos'});await page.locator('#local_area').selectOption({label:'Ikeja'});
 await expect(page.locator('#local_area')).not.toHaveAttribute('aria-invalid','true');
 const ids=(await page.locator('#local_area').getAttribute('aria-describedby')).split(' ');
 expect(ids).toContain('geography-help');for(const id of ids)await expect(page.locator('[id="'+id+'"]')).toHaveCount(1);
});

test('responsive hero selects a bounded candidate and reserves its layout',async({page},testInfo)=>{
 expect((await page.request.post('/admin/login',{form:{username:'browseradmin',password:'browser-test-password'}})).ok()).toBeTruthy();
 await page.goto('/admin/dashboard');await page.locator('input[name=landing_image]').setInputFiles({name:'hero.png',mimeType:'image/png',buffer:PNG});
 await page.locator('form[action="/admin/landing-image"] button[type=submit]').click();
 await page.setViewportSize({width:320,height:900});
 await page.addInitScript(()=>{
  window.mediaCLS=0;window.mediaLCP=0;
  new PerformanceObserver(list=>{for(const entry of list.getEntries())if(!entry.hadRecentInput)window.mediaCLS+=entry.value;}).observe({type:'layout-shift',buffered:true});
  new PerformanceObserver(list=>{for(const entry of list.getEntries())window.mediaLCP=entry.startTime;}).observe({type:'largest-contentful-paint',buffered:true});
 });
 await page.goto('/');const hero=page.locator('.landing-media');
 await expect(hero).toHaveAttribute('fetchpriority','high');await expect(hero).toHaveAttribute('loading','eager');
 await expect.poll(()=>hero.evaluate(img=>img.complete && img.naturalWidth)).toBe(320);
 expect(await hero.evaluate(img=>img.currentSrc)).toContain('/media/preview?');
 expect(await hero.evaluate(img=>img.currentSrc)).toContain('width=320');
 await checkPage(page,testInfo,'responsive-hero-320');
 const measurements=await page.evaluate(()=>({cls:window.mediaCLS,lcp:window.mediaLCP,images:performance.getEntriesByType('resource').filter(e=>e.name.includes('/media/preview?')).map(e=>({name:e.name,bytes:e.encodedBodySize,duration:e.duration}))}));
 expect(measurements.cls).toBeLessThanOrEqual(.1);expect(measurements.lcp).toBeLessThanOrEqual(5000);
 expect(measurements.images).toHaveLength(1);expect(measurements.images[0].bytes).toBeLessThanOrEqual(30*1024);
 await testInfo.attach('media-performance.json',{body:JSON.stringify(measurements,null,2),contentType:'application/json'});
 await page.setViewportSize({width:1280,height:900});await page.reload();
 await expect.poll(()=>hero.evaluate(img=>img.complete && img.naturalWidth > 0 && img.currentSrc.includes("width=1600"))).toBe(true);
});

test('public and customer discovery page through stable shop results',async({page},testInfo)=>{
 expect((await page.request.post('/admin/login',{form:{username:'browseradmin',password:'browser-test-password'}})).ok()).toBeTruthy();
 await page.goto('/admin/barbers/new');const csrf=await page.locator('input[name="csrf_token"]').first().inputValue();const suffix=Date.now();
 for(let i=0;i<25;i++){
  const result=await page.request.post('/admin/barbers/new',{form:{csrf_token:csrf,fullname:'Paging Barber '+i,phone:'08012345678',email:'paging-'+suffix+'-'+i+'@example.test',shopname:'Paging Shop '+i,shopaddress:'12 Example Street',state:'Osun',local_area:'Osogbo',password:'browser-pass-123'}});expect(result.ok(),await result.text()).toBeTruthy();
 }
 await page.goto('/explore?state=Osun&area=Osogbo');
 await expect(page.getByText('25 active shops in this location. Up to 24 shops per page.',{exact:true})).toBeVisible();
 await expect(page.locator('.barber-card')).toHaveCount(24);
 await page.getByRole('link',{name:'Next shops',exact:true}).click();const results=page.url();
 await expect(page.locator('.barber-card')).toHaveCount(1);await expect(page.getByRole('link',{name:'Next shops',exact:true})).toHaveCount(0);
 await page.getByRole('link',{name:'Paging Shop 24',exact:true}).click();
 await page.getByRole('link',{name:/Back to barber results/}).click();await expect(page).toHaveURL(results);
 await page.setViewportSize({width:320,height:900});await checkPage(page,testInfo,'public-paged-results');
 await page.getByRole('link',{name:'Previous shops',exact:true}).click();await expect(page.locator('.barber-card')).toHaveCount(24);
 const registered=await page.request.post('/customer/register',{multipart:{fullname:'Paging Customer',phone:'08012345678',email:'paging-customer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}});expect(registered.ok(),await registered.text()).toBeTruthy();
 await page.goto('/customer/barbers?state=Osun&area=Osogbo');await page.getByRole('link',{name:'Next shops',exact:true}).click();
 await expect(page.getByRole('link',{name:'Previous shops',exact:true})).toBeVisible();await checkPage(page,testInfo,'customer-paged-results');
});

test('catalogue uses bounded seller pages and exact-product continuation without JavaScript',async({page,browser},testInfo)=>{
 const suffix=Date.now(),email='catalogue-'+suffix+'@example.test',shop='Catalogue Page Shop '+suffix;
 const registered=await page.request.post('/barber/register',{multipart:{fullname:'Catalogue Seller',phone:'08012345678',email,shopname:shop,shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}});expect(registered.ok(),await registered.text()).toBeTruthy();
 expect((await page.request.post('/barber/login',{form:{email,password:'browser-pass-123'}})).ok()).toBeTruthy();
 await page.goto('/barber/products/new');const csrf=await page.locator('main input[name=csrf_token]').inputValue();
 for(let i=0;i<25;i++){
  const response=await page.request.post('/barber/products/new',{multipart:{csrf_token:csrf,name:'Catalogue Product '+i,category:'Care',description:'A product for the catalogue paging journey.',price:'1250.25',quantity:'2',available:'on'}});expect(response.ok(),await response.text()).toBeTruthy();
 }
 const context=await browser.newContext({javaScriptEnabled:false});
 try {
  const customer=await context.newPage();
  expect((await customer.request.post('/customer/register',{multipart:{fullname:'Catalogue Customer',phone:'08012345678',email:'catalogue-customer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}})).ok()).toBeTruthy();
  await customer.goto('/customer/barbers?state=Lagos&area=Ikeja');await customer.getByRole('link',{name:shop,exact:true}).first().click();
  const seller=customer.url().match(/\/barbers\/(\d+)/)[1];const catalogue='/customer/products?barber='+seller;
  await customer.goto(catalogue);await expect(customer.locator('.product-card')).toHaveCount(24);
  await customer.getByRole('link',{name:'Next page',exact:true}).click();await expect(customer.locator('.product-card')).toHaveCount(1);
  expect(new URL(customer.url()).searchParams.get('barber')).toBe(seller);
  const product=(await customer.locator('.product-card').getAttribute('id')).replace('product-','');
  await customer.setViewportSize({width:320,height:900});
  expect(await customer.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth+1)).toBeTruthy();
  await customer.screenshot({path:testInfo.outputPath('catalogue-native-second-page.png'),fullPage:true});
  // axe uses asynchronous JavaScript; inspect the same authenticated page in
  // a separate enabled context while keeping native navigation disabled.
  const inspection=await browser.newContext({storageState:await context.storageState(),viewport:{width:320,height:900}});
  try {const inspected=await inspection.newPage();await inspected.goto(customer.url());await checkPage(inspected,testInfo,'catalogue-second-page-accessibility');}finally{await inspection.close();}
  await customer.getByRole('link',{name:'Previous page',exact:true}).click();await expect(customer.locator('.product-card')).toHaveCount(24);
  await customer.goto(catalogue+'&product='+product);await expect(customer.locator('.product-card')).toHaveCount(1);
  await expect(customer.locator('.product-buy-form')).toHaveAttribute('action','/customer/products/'+product+'/add-to-cart?barber='+seller);
  await customer.goto(catalogue+'&after=2147483647');expect(new URL(customer.url()).searchParams.has('after')).toBe(false);await expect(customer.locator('.product-card')).toHaveCount(24);
 }finally{await context.close();}
});

test('both roles page message history chronologically and refresh to latest replies',async({browser},testInfo)=>{
 const customerContext=await browser.newContext(),barberContext=await browser.newContext();
 try {
  const customer=await customerContext.newPage(),barber=await barberContext.newPage(),suffix=Date.now(),email='history-barber-'+suffix+'@example.test',shop='History Shop '+suffix;
  expect((await barber.request.post('/barber/register',{multipart:{fullname:'History Barber',phone:'08012345678',email,shopname:shop,shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
  expect((await barber.request.post('/barber/login',{form:{email,password:'browser-pass-123'}})).ok()).toBeTruthy();
  expect((await customer.request.post('/customer/register',{multipart:{fullname:'History Customer',phone:'08012345678',email:'history-customer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}})).ok()).toBeTruthy();
  await customer.goto('/customer/barbers?state=Lagos&area=Ikeja');await customer.getByRole('link',{name:shop,exact:true}).first().click();await customer.getByRole('link',{name:'Message Barber',exact:true}).first().click();
  const action=await customer.locator('form.send').getAttribute('action'),csrf=await customer.locator('form.send input[name=csrf_token]').inputValue();
  for(let i=0;i<105;i++){const response=await customer.request.post(action,{maxRedirects:0,form:{csrf_token:csrf,text:'History '+String(i).padStart(3,'0')}});expect(response.status()).toBe(303);}
  await customer.goto(action);await expect(customer.locator('.thread .bubble')).toHaveCount(100);
  await expect(customer.locator('.thread .bubble').first()).toContainText('History 005');await expect(customer.locator('.thread .bubble').last()).toContainText('History 104');
  await customer.getByRole('link',{name:'Older messages',exact:true}).click();await expect(customer.locator('.thread .bubble')).toHaveCount(5);await expect(customer.locator('.thread .bubble').first()).toContainText('History 000');
  await customer.getByRole('link',{name:'Newer messages',exact:true}).click();await expect(customer.locator('.thread .bubble')).toHaveCount(100);
  await barber.goto('/barber/chats');await barber.getByRole('link').filter({hasText:'History Customer'}).first().click();
  await expect(barber.locator('.thread .bubble')).toHaveCount(100);await barber.getByRole('link',{name:'Older messages',exact:true}).click();await expect(barber.locator('.thread .bubble')).toHaveCount(5);
  await barber.getByRole('textbox',{name:'Your message'}).fill('Latest reply after paging');await barber.getByRole('button',{name:/send/i}).click();
  await expect(barber.getByText('Latest reply after paging',{exact:true})).toBeVisible();expect(new URL(barber.url()).searchParams.has('before')).toBe(false);
  await customer.getByRole('link',{name:'Refresh conversation',exact:true}).click();await expect(customer.getByText('Latest reply after paging',{exact:true})).toBeVisible();
  await customer.setViewportSize({width:320,height:900});
  const thread=customer.locator('.thread');await thread.focus();await customer.keyboard.press('Home');await expect.poll(()=>thread.evaluate(el=>el.scrollTop)).toBe(0);
  await customer.keyboard.press('End');await expect.poll(()=>thread.evaluate(el=>el.scrollTop)).toBeGreaterThan(0);
  await checkPage(customer,testInfo,'paged-history-mobile');
 }finally{await customerContext.close();await barberContext.close();}
});

test('admin account search reaches older pages without JavaScript',async({page,browser},testInfo)=>{
 expect((await page.request.post('/admin/login',{form:{username:'browseradmin',password:'browser-test-password'}})).ok()).toBeTruthy();
 await page.goto('/admin/barbers/new');const csrf=await page.locator('main input[name=csrf_token]').inputValue(),suffix=Date.now();
 for(let i=0;i<25;i++){
  const response=await page.request.post('/admin/barbers/new',{maxRedirects:0,form:{csrf_token:csrf,fullname:'Account Paging '+suffix+' '+i,phone:'08012345678',email:'admin-paging-'+suffix+'-'+i+'@example.test',shopname:'Account Shop '+i,shopaddress:'12 Example Street',state:'Osun',local_area:'Osogbo',password:'browser-pass-123'}});expect(response.status()).toBe(303);
 }
 const context=await browser.newContext({javaScriptEnabled:false,storageState:await page.context().storageState()});
 try{
  const native=await context.newPage();await native.goto('/admin/barbers');await expect(native.locator('tbody tr')).toHaveCount(24);
  await expect(native.getByRole('link',{name:'Account Paging '+suffix+' 0',exact:true})).toHaveCount(0);
  await native.getByRole('link',{name:'Next page',exact:true}).click();await expect(native.getByRole('link',{name:'Account Paging '+suffix+' 0',exact:true})).toBeVisible();
  await native.getByRole('link',{name:'Previous page',exact:true}).click();await native.getByRole('searchbox',{name:'Search barbers',exact:true}).fill('admin-paging-'+suffix+'-0@');await native.getByRole('button',{name:'Search barbers',exact:true}).click();
  await expect(native.locator('tbody tr')).toHaveCount(1);await expect(native.getByRole('link',{name:'Account Paging '+suffix+' 0',exact:true})).toBeVisible();
  await native.getByRole('searchbox',{name:'Search barbers',exact:true}).fill('absent-account-'+suffix);await native.getByRole('button',{name:'Search barbers',exact:true}).click();await expect(native.getByText('No barber accounts match this page or search.')).toBeVisible();
  await page.setViewportSize({width:320,height:900});await page.goto('/admin/barbers?q=admin-paging-'+suffix);await checkPage(page,testInfo,'admin-account-pages-320');
  await page.goto('/admin/customers');await expect(page.getByRole('searchbox',{name:'Search customers',exact:true})).toBeVisible();await checkPage(page,testInfo,'admin-customer-pages-320');
  expect((await page.request.get('/admin/barbers?q='+encodeURIComponent('x'.repeat(201)))).status()).toBe(400);
 }finally{await context.close();}
});

test('inventory and portfolio pages preserve independent cursors and selected services',async({page,browser},testInfo)=>{
 const suffix=Date.now(),email='portfolio-'+suffix+'@example.test',shop='Portfolio Pages '+suffix;
 expect((await page.request.post('/barber/register',{multipart:{fullname:'Portfolio Seller',phone:'08012345678',email,shopname:shop,shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
 expect((await page.request.post('/barber/login',{form:{email,password:'browser-pass-123'}})).ok()).toBeTruthy();
 await page.goto('/barber/hairstyles/new');const csrf=await page.locator('main input[name=csrf_token]').inputValue();
 for(let i=0;i<25;i++){
  expect((await page.request.post('/barber/hairstyles/new',{maxRedirects:0,multipart:{csrf_token:csrf,name:'Portfolio Style '+i,category:'Style',description:'A service for portfolio paging.',price:'1250.25',duration:'30',available:'on'}})).status()).toBe(303);
  expect((await page.request.post('/barber/products/new',{maxRedirects:0,multipart:{csrf_token:csrf,name:'Portfolio Product '+i,category:'Care',description:'A product for portfolio paging.',price:'1250.25',quantity:'2',available:'on'}})).status()).toBe(303);
 }
 await page.goto('/barber/hairstyles');await expect(page.locator('.barber-hairstyle-card')).toHaveCount(24);await page.getByRole('link',{name:'Next page',exact:true}).click();await expect(page.locator('.barber-hairstyle-card')).toHaveCount(1);
 const style=(await page.getByRole('link',{name:'Portfolio Style 24',exact:true}).getAttribute('href')).match(/\d+$/)[0];
 await page.goto('/barber/products');await expect(page.locator('.product-card')).toHaveCount(24);await page.getByRole('link',{name:'Next page',exact:true}).click();await expect(page.locator('.product-card')).toHaveCount(1);
 const context=await browser.newContext({javaScriptEnabled:false});
 try{
  const customer=await context.newPage();expect((await customer.request.post('/customer/register',{multipart:{fullname:'Portfolio Customer',phone:'08012345678',email:'portfolio-customer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}})).ok()).toBeTruthy();
  await customer.goto('/customer/barbers?state=Lagos&area=Ikeja');await customer.getByRole('link',{name:shop,exact:true}).first().click();const seller=customer.url().match(/\/barbers\/(\d+)/)[1];
  await customer.goto('/customer/barbers/'+seller+'?state=Lagos&area=Ikeja&after=41&hairstyle='+style);await expect(customer.locator('#availability-service option')).toHaveCount(25);await expect(customer.locator('[data-booking-summary]')).toContainText('Portfolio Style 24');
  await customer.getByRole('link',{name:'Next services',exact:true}).click();await expect(customer.locator('#availability-service option')).toHaveCount(1);expect(new URL(customer.url()).searchParams.get('after')).toBe('41');
  await customer.getByRole('link',{name:'Next products',exact:true}).click();expect(new URL(customer.url()).searchParams.has('services_after')).toBe(true);expect(new URL(customer.url()).searchParams.has('products_after')).toBe(true);
  await customer.getByRole('button',{name:'Show available times',exact:true}).click();await expect(customer.locator('#availability-service option')).toHaveCount(1);await expect(customer.locator('[data-booking-summary]')).toContainText('Portfolio Style 24');
  const portfolioURL=customer.url(),buyerCsrf=await customer.locator('#booking input[name=csrf_token]').inputValue();
  await customer.goto('/customer/products?barber='+seller);
  for(const action of await customer.locator('.product-buy-form').evaluateAll(forms=>forms.map(form=>form.getAttribute('action')))){expect((await customer.request.post(action,{maxRedirects:0,form:{csrf_token:buyerCsrf}})).status()).toBe(303);}
  await customer.getByRole('link',{name:'Next page',exact:true}).click();expect((await customer.request.post(await customer.locator('.product-buy-form').getAttribute('action'),{maxRedirects:0,form:{csrf_token:buyerCsrf}})).status()).toBe(303);
  await customer.goto('/customer/cart');await expect(customer.locator('.cart-item')).toHaveCount(24);await expect(customer.getByText('₦31,256.25',{exact:true})).toBeVisible();
  await customer.getByRole('link',{name:'Next page',exact:true}).click();await expect(customer.locator('.cart-item')).toHaveCount(1);await expect(customer.getByText('₦31,256.25',{exact:true})).toBeVisible();
  await customer.getByRole('button',{name:/Place order/}).click();await expect(customer).toHaveURL(/order-confirmation/);await expect(customer.getByRole('heading',{name:'Portfolio Product 0 Pending',exact:true})).toBeVisible();await expect(customer.getByRole('heading',{name:'Portfolio Product 24 Pending',exact:true})).toBeVisible();
  await customer.goto(portfolioURL);
  await page.goto('/explore/'+seller+'?state=Lagos&area=Ikeja&after=41');await expect(page.locator('.public-style-card')).toHaveCount(24);await page.getByRole('link',{name:'Next services',exact:true}).click();await expect(page.locator('.public-style-card')).toHaveCount(1);
  await page.setViewportSize({width:320,height:900});await checkPage(page,testInfo,'public-portfolio-pages-320');
  const inspection=await browser.newContext({storageState:await context.storageState(),viewport:{width:320,height:900}});try{const inspected=await inspection.newPage();await inspected.goto(customer.url());await checkPage(inspected,testInfo,'customer-portfolio-pages-320');}finally{await inspection.close();}
 }finally{await context.close();}
});

test('booking and order histories remain bounded with complete dashboard totals',async({page,browser},testInfo)=>{
 const suffix=Date.now(),email='bounded-history-'+suffix+'@example.test',shop='Bounded History '+suffix;
 expect((await page.request.post('/barber/register',{multipart:{fullname:'History Seller',phone:'08012345678',email,shopname:shop,shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',password:'browser-pass-123',confirm_password:'browser-pass-123',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
 expect((await page.request.post('/barber/login',{form:{email,password:'browser-pass-123'}})).ok()).toBeTruthy();await page.goto('/barber/hairstyles/new');const sellerCsrf=await page.locator('main input[name=csrf_token]').inputValue();
 expect((await page.request.post('/barber/hairstyles/new',{multipart:{csrf_token:sellerCsrf,name:'History Service',category:'Style',description:'History appointment service.',price:'1250.25',duration:'30',available:'on'}})).ok()).toBeTruthy();
 expect((await page.request.post('/barber/products/new',{multipart:{csrf_token:sellerCsrf,name:'History Product',category:'Care',description:'History product.',price:'1250.25',quantity:'40',available:'on'}})).ok()).toBeTruthy();
 const context=await browser.newContext({javaScriptEnabled:false});try{
 const customer=await context.newPage();expect((await customer.request.post('/customer/register',{multipart:{fullname:'History Buyer',phone:'08012345678',email:'history-buyer-'+suffix+'@example.test',password:'browser-pass-123',confirm_password:'browser-pass-123'}})).ok()).toBeTruthy();
 await customer.goto('/customer/barbers?state=Lagos&area=Ikeja');await customer.getByRole('link',{name:shop,exact:true}).first().click();const seller=customer.url().match(/\/barbers\/(\d+)/)[1],style=await customer.locator('#availability-service').inputValue(),csrf=await customer.locator('#booking input[name=csrf_token]').inputValue();
 await customer.goto('/customer/products?barber='+seller);const addAction=await customer.locator('.product-buy-form').getAttribute('action');
 for(let i=0;i<25;i++){
 const day=new Date();day.setUTCDate(day.getUTCDate()+7+i);const date=day.toISOString().slice(0,10);
 const response=await customer.request.post('/customer/book',{maxRedirects:0,form:{csrf_token:csrf,barber_id:seller,hairstyle_id:style,date,time:'10:00',notes:'History booking '+i}});expect(response.status(),await response.text()).toBe(303);
 expect((await customer.request.post(addAction,{maxRedirects:0,form:{csrf_token:csrf}})).status()).toBe(303);
 expect((await customer.request.post('/customer/products/checkout',{maxRedirects:0,form:{csrf_token:csrf,checkout_key:(i+1).toString(16).padStart(32,'0')}})).status()).toBe(303);
 }
 await customer.goto('/customer/bookings');await expect(customer.locator('.booking-card')).toHaveCount(24);await customer.getByRole('link',{name:'Next page',exact:true}).click();await expect(customer.locator('.booking-card')).toHaveCount(1);
 await customer.getByRole('combobox',{name:'Appointment group'}).selectOption('finished');await customer.getByRole('button',{name:'Show appointments',exact:true}).click();await expect(customer.locator('.booking-card')).toHaveCount(0);
 await customer.goto('/customer/dashboard');await expect(customer.locator('.appointment-list').first().locator('.appointment-card')).toHaveCount(6);await expect(customer.getByRole('link',{name:'Next orders',exact:true})).toBeVisible();await customer.getByRole('link',{name:'Next orders',exact:true}).click();await expect(customer.getByRole('link',{name:/View order/i})).toHaveCount(1);
 await page.goto('/barber/dashboard');await expect(page.locator('.barber-section:has(#orders) tbody tr')).toHaveCount(24);await page.getByRole('link',{name:'Next orders',exact:true}).click();await expect(page.locator('.barber-section:has(#orders) tbody tr')).toHaveCount(1);
 await page.getByRole('combobox',{name:'Fulfillment state',exact:true}).selectOption('Collected');await page.getByRole('button',{name:'Show order items',exact:true}).click();await expect(page.locator('.barber-section:has(#orders) tbody tr')).toHaveCount(0);
 await page.getByRole('combobox',{name:'Fulfillment state',exact:true}).selectOption('Pending');await page.getByRole('button',{name:'Show order items',exact:true}).click();await expect(page.locator('.barber-section:has(#orders) tbody tr')).toHaveCount(24);
 await page.setViewportSize({width:320,height:900});await checkPage(page,testInfo,'seller-orders-paged-320');
 const inspection=await browser.newContext({storageState:await context.storageState(),viewport:{width:320,height:900}});try{const inspected=await inspection.newPage();await inspected.goto(customer.url());await checkPage(inspected,testInfo,'customer-orders-paged-320');}finally{await inspection.close();}
 }finally{await context.close();}
});
