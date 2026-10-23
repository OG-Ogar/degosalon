const {test,expect}=require('@playwright/test');
const {default:AxeBuilder}=require('@axe-core/playwright');
const PNG=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFklEQVR4nGOUs9FgYGBgYmBgYGBgAAAGOACGNuhQ0wAAAABJRU5ErkJggg==','base64');
const password='browser-pass-123';
const longText='A detailed service description for international customers. '+ 'LongContent'.repeat(18)+' — café, coupe et soins.';
async function assertAccessible(page,testInfo,name,{axe=true}={}){
 await expect(page.locator('h1').first()).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),name+' horizontal overflow').toBeTruthy();
 if(axe){const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa']).analyze();expect(result.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})),name).toEqual([]);}
 await page.keyboard.press('Tab');
 const focus=await page.evaluate(()=>{const el=document.activeElement,style=getComputedStyle(el),box=el.getBoundingClientRect();return {tag:el.tagName,visible:box.width>0&&box.height>0,indicator:style.outlineStyle!=='none'&&parseFloat(style.outlineWidth)>0||style.boxShadow!=='none'};});
 expect(focus.tag,name+' keyboard focus').not.toBe('BODY');expect(focus.visible,name+' focused control visibility').toBeTruthy();expect(focus.indicator,name+' focus indicator').toBeTruthy();
 const small=await page.locator('button,select,input:not([type=hidden]):not([type=checkbox]):not([type=radio]),textarea').evaluateAll(elements=>elements.filter(el=>{const b=el.getBoundingClientRect();return b.width>0&&b.height>0&&!el.disabled&&(b.width<24||b.height<24)}).map(el=>({tag:el.tagName,name:el.getAttribute('name'),text:el.textContent.slice(0,40)})));
 expect(small,name+' minimum control target').toEqual([]);
 const fields=page.locator('main input:not([type=hidden]),main select,main textarea');
 for(let index=0;index<Math.min(3,await fields.count());index++){const field=fields.nth(index);if(!await field.isVisible()||!await field.isEnabled())continue;await field.evaluate(el=>el.scrollIntoView({block:'center',inline:'nearest'}));await field.evaluate(el=>el.focus({preventScroll:true}));await expect.poll(()=>field.evaluate(el=>{const style=getComputedStyle(el),box=el.getBoundingClientRect();return {focused:document.activeElement===el,outlined:style.outlineStyle!=='none'&&parseFloat(style.outlineWidth)>=2,inView:box.top>=-1&&box.bottom<=innerHeight+1};}),{message:name+' field focus visibility'}).toEqual({focused:true,outlined:true,inView:true});}

 await page.screenshot({path:testInfo.outputPath(name+'.png'),fullPage:true});
}
// Give each isolated client its own trusted synthetic address so unrelated
// scenarios do not consume one another's authentication budgets.
let acceptanceClientAddress=1;
function acceptanceContext(browser,options={}){const address=acceptanceClientAddress;acceptanceClientAddress=address===254?1:address+1;return browser.newContext({...options,extraHTTPHeaders:{'X-Forwarded-For':'198.51.100.'+address,...options.extraHTTPHeaders}});}
let fixture;
test.beforeAll(async({browser})=>{
 test.setTimeout(120000);
 const barberContext=await acceptanceContext(browser),customerContext=await acceptanceContext(browser),adminContext=await acceptanceContext(browser);
 try{
 const barber=await barberContext.newPage(),customer=await customerContext.newPage(),admin=await adminContext.newPage(),suffix=Date.now();
 const email='acceptance-barber-'+suffix+'@example.test',shop='Acceptance atelier with a deliberately long shop name '+suffix;
 expect((await barber.request.post('/barber/register',{multipart:{fullname:'Acceptance Barber with a long international name',phone:'08012345678',email,shopname:shop,shopaddress:'12 Example Street, a detailed address for customers',state:'Lagos',local_area:'Ikeja',password,confirm_password:password,personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG},profile_image:{name:'shop.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
 expect((await barber.request.post('/barber/login',{form:{email,password}})).ok()).toBeTruthy();
 await barber.goto('/barber/hairstyles/new');const sellerCsrf=await barber.locator('main input[name=csrf_token]').inputValue();
 expect((await barber.request.post('/barber/hairstyles/new',{multipart:{csrf_token:sellerCsrf,name:'Acceptance service with a long descriptive name',category:'Style',description:longText,price:'1250.25',duration:'30',available:'on',image:{name:'service.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
 await barber.goto('/barber/hairstyles');const style=(await barber.getByRole('link',{name:'Acceptance service with a long descriptive name',exact:true}).getAttribute('href')).match(/\d+$/)[0];
 expect((await barber.request.post('/barber/products/new',{multipart:{csrf_token:sellerCsrf,name:'Acceptance product with a long descriptive name',category:'Care',description:longText,price:'1250.25',quantity:'8',available:'on',image:{name:'product.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
 expect((await customer.request.post('/customer/register',{multipart:{fullname:'Acceptance Customer with a long international name',phone:'08012345678',email:'acceptance-customer-'+suffix+'@example.test',password,confirm_password:password,profile_image:{name:'customer.png',mimeType:'image/png',buffer:PNG}}})).ok()).toBeTruthy();
 await customer.goto('/customer/barbers?state=Lagos&area=Ikeja');await customer.getByRole('link',{name:shop,exact:true}).first().click();const seller=customer.url().match(/\/barbers\/(\d+)/)[1],csrf=await customer.locator('#booking input[name=csrf_token]').inputValue();
 const day=new Date();day.setUTCDate(day.getUTCDate()+8);const date=day.toISOString().slice(0,10);
 const booked=await customer.request.post('/customer/book',{maxRedirects:0,form:{csrf_token:csrf,barber_id:seller,hairstyle_id:style,date,time:'10:00',notes:longText}});expect(booked.status()).toBe(303);const bookingPath=booked.headers().location,booking=bookingPath.match(/\d+$/)[0];
 await customer.goto('/customer/products?barber='+seller);const action=await customer.locator('.product-buy-form').getAttribute('action'),product=action.match(/products\/(\d+)/)[1];expect((await customer.request.post(action,{form:{csrf_token:csrf}})).ok()).toBeTruthy();
 const purchased=await customer.request.post('/customer/products/checkout',{maxRedirects:0,form:{csrf_token:csrf,checkout_key:BigInt(suffix).toString(16).padStart(32,'0')}});expect(purchased.status()).toBe(303);const orderPath=purchased.headers().location;
 // Keep a line in the cart so acceptance covers its populated form.
 expect((await customer.request.post(action,{form:{csrf_token:csrf}})).ok()).toBeTruthy();
 await customer.goto('/customer/chats/'+seller);await customer.getByRole('textbox',{name:'Your message'}).fill(longText);await customer.getByRole('button',{name:/send/i}).click();
 await barber.goto('/barber/chats');await barber.getByRole('link').filter({hasText:'Acceptance Customer with a long international name'}).first().click();const sellerChat=barber.url().replace('http://127.0.0.1:8080','');
 expect((await admin.request.post('/admin/login',{form:{username:'browseradmin',password:'browser-test-password'}})).ok()).toBeTruthy();
 fixture={email,seller,style,product,booking,bookingPath,orderPath,date,sellerChat,sellerCsrf,csrf,checkoutKey:BigInt(suffix).toString(16).padStart(32,'0'),barberState:await barberContext.storageState(),customerState:await customerContext.storageState(),adminState:await adminContext.storageState()};
 }finally{await barberContext.close();await customerContext.close();await adminContext.close();}
});

const families={
 public:()=>['/','/explore','/explore/'+fixture.seller,'/help','/support','/privacy','/terms','/missing-page'],
 auth:()=>['/customer/login','/customer/register','/customer/forgot-password','/customer/reset-password?token=invalid','/barber/login','/barber/register','/barber/forgot-password','/barber/reset-password?token=invalid','/admin/login'],
 customer:()=>['/customer/dashboard','/customer/barbers','/customer/barbers/'+fixture.seller,'/customer/bookings',fixture.bookingPath,'/customer/products?barber='+fixture.seller,'/customer/cart',fixture.orderPath,'/customer/profile','/customer/profile/edit','/customer/chats','/customer/chats/'+fixture.seller],
 barber:()=>['/barber/dashboard','/barber/bookings','/barber/hairstyles','/barber/hairstyles/new','/barber/hairstyles/'+fixture.style,'/barber/hairstyles/'+fixture.style+'/edit','/barber/hairstyles/'+fixture.style+'/delete','/barber/products','/barber/products/new','/barber/products/'+fixture.product+'/edit','/barber/profile','/barber/profile/edit','/barber/schedule','/barber/chats',()=>fixture.sellerChat],
 admin:()=>['/admin/dashboard','/admin/barbers','/admin/barbers/new','/admin/barbers/'+fixture.seller,'/admin/barbers/'+fixture.seller+'/edit','/admin/barbers/'+fixture.seller+'/delete','/admin/customers','/admin/bookings']
};
for(const [family,routes] of Object.entries(families)){
 test('acceptance matrix: '+family,async({browser},testInfo)=>{
 test.setTimeout(240000);
 const state=fixture[family+'State'];const context=await acceptanceContext(browser,{storageState:state});
 try{const page=await context.newPage();const metrics=await context.newCDPSession(page);for(const routeOrFn of routes()){
 const route=typeof routeOrFn==='function'?routeOrFn():routeOrFn,name=family+'-'+route.split('?')[0].replaceAll('/','-');
 for(const [width,label] of [[1280,'desktop'],[320,'phone'],[640,'zoom-reflow']]){
 await page.setViewportSize({width,height:width===640?450:900});await metrics.send('Emulation.setDeviceMetricsOverride',{width,height:width===640?450:900,deviceScaleFactor:width===640?2:1,mobile:false});const response=await page.goto(route);expect(response.status()).toBe(route==='/missing-page'?404:route.includes('token=invalid')?400:200);
 await assertAccessible(page,testInfo,name+'-'+label,{axe:width!==640});
 }
 }}finally{await context.close();}
 });
}

test('touch navigation and broken media retain usable populated views',async({browser},testInfo)=>{
 for(const [family,routes] of Object.entries({public:['/explore/'+fixture.seller],customer:['/customer/barbers/'+fixture.seller,'/customer/products?barber='+fixture.seller,'/customer/cart'],barber:['/barber/hairstyles'],admin:['/admin/barbers/'+fixture.seller]})){
 const context=await acceptanceContext(browser,{storageState:fixture[family+'State'],viewport:{width:320,height:900},isMobile:true,hasTouch:true});
 try{await context.route('**/*',route=>route.request().resourceType()==='image'?route.abort():route.continue());const page=await context.newPage();
 for(const route of routes){await page.goto(route);const images=page.locator('main img:not(.image-viewer-image)');expect(await images.count()).toBeGreaterThan(0);await expect.poll(()=>images.evaluateAll(images=>images.every(img=>img.complete&&img.naturalWidth===0))).toBe(true);
 const missingAlt=await images.evaluateAll(images=>images.filter(img=>!img.getAttribute('alt')&&img.getAttribute('aria-hidden')!=='true').map(img=>img.className));expect(missingAlt).toEqual([]);
 await assertAccessible(page,testInfo,family+'-broken-media-'+route.split('?')[0].replaceAll('/','-'));
 if(family==='public'){const photo=page.locator('main img.image-zoomable').first();await photo.tap();await expect(page.getByRole('dialog',{name:'Full-size image'})).toBeVisible();await page.getByRole('button',{name:'Close image',exact:true}).tap();await expect(page.getByRole('dialog')).not.toBeVisible();}
 }
 }finally{await context.close();}
 }
});

test('booking outcomes, schedule conflicts, verified reviews and fulfillment persist',async({browser},testInfo)=>{
 const sellerContext=await acceptanceContext(browser,{storageState:fixture.barberState}),buyerContext=await acceptanceContext(browser,{storageState:fixture.customerState});
 try{
 const seller=await sellerContext.newPage(),buyer=await buyerContext.newPage();
 const replay=await buyer.request.post('/customer/products/checkout',{maxRedirects:0,form:{csrf_token:fixture.csrf,checkout_key:fixture.checkoutKey}});expect(replay.status()).toBe(303);expect(replay.headers().location).toBe(fixture.orderPath);
 expect((await buyer.request.post('/customer/barbers/'+fixture.seller+'/reviews',{maxRedirects:0,form:{csrf_token:fixture.csrf,rating:'5',comment:'Premature review'}})).status()).toBe(403);
 await seller.goto('/barber/bookings');await seller.locator('form[action="/barber/bookings/'+fixture.booking+'/accept"] button').click();await expect(seller.locator('form[action$="/complete"]')).toHaveCount(1);
 await seller.goto('/barber/schedule');const form=await seller.locator('main form').evaluate(form=>Object.fromEntries(new FormData(form)));const weekday=new Date(fixture.date+'T12:00:00Z').getUTCDay();
 const rejected=await seller.request.post('/barber/schedule',{maxRedirects:0,form:{...form,['day_'+weekday+'_closed']:'yes'}});expect(rejected.status()).toBe(409);expect(await rejected.text()).toContain('overlaps an existing appointment');
 await seller.goto('/barber/schedule');await expect(seller.locator('[name="day_'+weekday+'_closed"]')).not.toBeChecked();
 await seller.goto('/barber/bookings');await seller.locator('form[action="/barber/bookings/'+fixture.booking+'/complete"] button').click();await expect(seller.getByText('Completed',{exact:true})).toBeVisible();
 await buyer.goto('/customer/barbers/'+fixture.seller);await buyer.getByRole('combobox',{name:'Rating',exact:true}).selectOption('5');await buyer.getByRole('textbox',{name:'Comment',exact:true}).fill(longText);await buyer.getByRole('button',{name:'Post Review',exact:true}).click();await expect(buyer.getByText(longText,{exact:true}).first()).toBeVisible();await expect(buyer.getByRole('button',{name:'Post Review',exact:true})).toHaveCount(0);
 expect((await buyer.request.post('/customer/barbers/'+fixture.seller+'/reviews',{maxRedirects:0,form:{csrf_token:fixture.csrf,rating:'5',comment:'Duplicate'}})).status()).toBe(403);
 await seller.goto('/barber/schedule');const dayFields=seller.locator('fieldset').nth(weekday);await dayFields.locator('[name$="break_start"]').fill('12:00');await dayFields.locator('[name$="break_end"]').fill('13:00');await seller.getByRole('button',{name:'Save schedule',exact:true}).click();await expect(seller).toHaveURL(/saved=1/);
 await buyer.goto('/customer/barbers/'+fixture.seller+'?hairstyle='+fixture.style+'&date='+fixture.date);await expect(buyer.locator('#time option[value="12:00"]')).toHaveCount(0);
 await seller.goto('/barber/dashboard');await seller.getByRole('button',{name:'Mark ready',exact:true}).click();await expect(seller.getByRole('button',{name:'Mark collected',exact:true})).toBeVisible();await seller.getByRole('button',{name:'Mark collected',exact:true}).click();await expect(seller.locator('.domain-status').filter({hasText:'Collected'}).first()).toBeVisible();
 await buyer.goto(fixture.orderPath);await expect(buyer.locator('.domain-status').filter({hasText:'Collected'}).first()).toBeVisible();await buyer.goto('/customer/cart');await buyer.getByRole('button',{name:/Place order/}).click();const cancelledOrder=buyer.url();await seller.goto('/barber/dashboard');await seller.getByRole('button',{name:'Cancel item and restore stock',exact:true}).click();await buyer.goto(cancelledOrder);await expect(buyer.getByText('Cancelled',{exact:true}).first()).toBeVisible();await seller.goto('/barber/products/'+fixture.product+'/edit');await expect(seller.locator('[name=quantity]')).toHaveValue('7');await seller.goto('/barber/dashboard');await buyer.goto(fixture.bookingPath);await expect(buyer.getByText('This appointment has been marked completed.',{exact:true})).toBeVisible();
 for(const [page,name] of [[buyer,'completed-booking'],[seller,'collected-sale']]){await page.setViewportSize({width:320,height:900});await assertAccessible(page,testInfo,name);}
 }finally{await sellerContext.close();await buyerContext.close();}
});

test('both roles reset credentials, revoke prior sessions and reject reused links',async({browser},testInfo)=>{
 test.setTimeout(240000);
 for(const role of ['customer','barber']){
 const context=await acceptanceContext(browser,{extraHTTPHeaders:{'X-Forwarded-For':'198.51.100.'+(role==='customer'?'21':'22')}});let previousContext;
 try{
 const page=await context.newPage(),suffix=Date.now(),email='recovery-'+role+'-'+suffix+'@example.test';
 const fields={fullname:'Recovery '+role,phone:'08012345678',email,password,confirm_password:password};if(role==='barber')Object.assign(fields,{shopname:'Recovery Shop '+suffix,shopaddress:'12 Example Street',state:'Lagos',local_area:'Ikeja',personal_image:{name:'profile.png',mimeType:'image/png',buffer:PNG}});
 expect((await page.request.post('/'+role+'/register',{multipart:fields})).ok()).toBeTruthy();if(role==='barber')expect((await page.request.post('/barber/login',{form:{email,password}})).ok()).toBeTruthy();
 if(role==='customer')expect((await page.request.get(fixture.orderPath)).status()).toBe(404);
 previousContext=await acceptanceContext(browser,{storageState:await context.storageState(),extraHTTPHeaders:{'X-Forwarded-For':'198.51.100.'+(role==='customer'?'21':'22')}});const previous=await previousContext.newPage();
 await page.goto('/'+role+'/forgot-password');await page.getByRole('textbox',{name:'Email address',exact:true}).fill(email);await page.getByRole('button',{name:'Send reset link',exact:true}).click();await page.getByRole('link',{name:'Choose a new password',exact:true}).click();const resetURL=page.url();
 for(const width of [320,640]){await page.setViewportSize({width,height:900});await assertAccessible(page,testInfo,role+'-valid-reset-'+width);}
 await page.getByLabel('New password',{exact:true}).fill('replacement-pass-123');await page.getByLabel('Confirm new password',{exact:true}).fill('different-pass-123');await page.getByLabel('Confirm new password',{exact:true}).press('Enter');await expect(page.locator('#form-error')).toBeVisible();
 await page.getByLabel('New password',{exact:true}).fill('replacement-pass-123');await page.getByLabel('Confirm new password',{exact:true}).fill('replacement-pass-123');await page.getByLabel('Confirm new password',{exact:true}).press('Enter');await expect(page).toHaveURL(new RegExp('/'+role+'/login'));
 const revoked=await previous.request.get('/'+role+'/dashboard',{maxRedirects:0});expect(revoked.status()).toBeGreaterThanOrEqual(300);expect(revoked.status()).toBeLessThan(400);expect(revoked.headers().location).toContain('/'+role+'/login');
 const reused=await page.goto(resetURL);expect(reused.status()).toBe(400);await expect(page.getByRole('button',{name:'Save new password',exact:true})).toHaveCount(0);
 const oldLogin=await page.request.post('/'+role+'/login',{form:{email,password}});expect(oldLogin.ok()).toBe(false);expect((await page.request.post('/'+role+'/login',{form:{email,password:'replacement-pass-123'}})).ok()).toBeTruthy();await page.goto('/'+role+'/dashboard');await expect(page.locator('h1').first()).toBeVisible();
 }finally{if(previousContext)await previousContext.close();await context.close();}
 }
});

test('account suspension and archival hide shops while retaining completed history',async({browser},testInfo)=>{
 const adminContext=await acceptanceContext(browser,{storageState:fixture.adminState}),sellerContext=await acceptanceContext(browser,{storageState:fixture.barberState}),buyerContext=await acceptanceContext(browser,{storageState:fixture.customerState});
 try{
 const admin=await adminContext.newPage(),seller=await sellerContext.newPage(),buyer=await buyerContext.newPage();for(const action of ['accept','complete']){const response=await seller.request.post('/barber/bookings/'+fixture.booking+'/'+action,{maxRedirects:0,form:{csrf_token:fixture.sellerCsrf}});expect([303,409]).toContain(response.status());}await seller.goto('/barber/dashboard');while(await seller.getByRole('button',{name:'Mark ready',exact:true}).count())await seller.getByRole('button',{name:'Mark ready',exact:true}).first().click();while(await seller.getByRole('button',{name:'Mark collected',exact:true}).count())await seller.getByRole('button',{name:'Mark collected',exact:true}).first().click();await admin.goto('/admin/barbers?q='+encodeURIComponent(fixture.email));const csrf=await admin.locator('form[action="/admin/barbers/'+fixture.seller+'/suspend"] input[name=csrf_token]').inputValue();await admin.locator('form[action="/admin/barbers/'+fixture.seller+'/suspend"] button').click();
 expect((await buyer.request.get('/explore/'+fixture.seller)).status()).toBe(404);const invalidated=await seller.request.get('/barber/dashboard',{maxRedirects:0});expect(invalidated.headers().location).toContain('/barber/login');
 expect((await admin.request.post('/admin/barbers/'+fixture.seller+'/activate',{form:{csrf_token:csrf}})).ok()).toBeTruthy();expect((await buyer.request.get('/explore/'+fixture.seller)).status()).toBe(200);
 await admin.goto('/admin/barbers/'+fixture.seller+'/delete');await Promise.all([admin.waitForURL(url=>url.pathname==='/admin/barbers'),admin.getByRole('button',{name:'Yes, archive this barber',exact:true}).click()]);expect((await buyer.request.get('/explore/'+fixture.seller)).status()).toBe(404);
 await buyer.goto(fixture.bookingPath);await expect(buyer.getByText('This appointment has been marked completed.',{exact:true})).toBeVisible();await buyer.goto(fixture.orderPath);await expect(buyer.getByRole('heading',{name:/Acceptance product/})).toBeVisible();await buyer.setViewportSize({width:320,height:900});await assertAccessible(buyer,testInfo,'archived-seller-receipt');
 }finally{await adminContext.close();await sellerContext.close();await buyerContext.close();}
});


test('authentication peer budget throttles one client without blocking another', async ({browser}) => {
 const limited = await acceptanceContext(browser, {extraHTTPHeaders: {'X-Forwarded-For': '198.51.100.90'}});
 const other = await acceptanceContext(browser, {extraHTTPHeaders: {'X-Forwarded-For': '198.51.100.91'}});
 const invalidForm = index => ({fullname: '', email: 'peer-budget-' + index + '@example.test'});
 try {
  // Distinct identities isolate the 20-attempt peer budget from the
  // independent five-attempt account budget. Invalid forms create no users.
  for (let i = 0; i < 20; i++) {
   expect((await limited.request.post('/customer/register', {form: invalidForm(i)})).status()).toBe(400);
  }
  const rejected = await limited.request.post('/customer/register', {form: invalidForm(20)});
  expect(rejected.status()).toBe(429);
  expect(Number(rejected.headers()['retry-after'])).toBeGreaterThan(0);
  expect(rejected.headers()['cache-control']).toBe('no-store');
  expect((await other.request.post('/customer/register', {form: invalidForm(21)})).status()).toBe(400);
 } finally {
  await limited.close();
  await other.close();
 }
});

test('authentication account budget follows the identity across clients', async ({browser}) => {
 const first = await acceptanceContext(browser, {extraHTTPHeaders: {'X-Forwarded-For': '198.51.100.92'}});
 const second = await acceptanceContext(browser, {extraHTTPHeaders: {'X-Forwarded-For': '198.51.100.93'}});
 const form = {fullname: '', email: 'account-budget@example.test'};
 try {
  for (let i = 0; i < 5; i++) {
   expect((await first.request.post('/customer/register', {form})).status()).toBe(400);
  }
  const rejected = await second.request.post('/customer/register', {form});
  expect(rejected.status()).toBe(429);
  expect(Number(rejected.headers()['retry-after'])).toBeGreaterThan(0);
  expect(rejected.headers()['cache-control']).toBe('no-store');
  expect((await second.request.post('/customer/register', {
   form: {fullname: '', email: 'another-account-budget@example.test'}
  })).status()).toBe(400);
 } finally {
  await first.close();
  await second.close();
 }
});
