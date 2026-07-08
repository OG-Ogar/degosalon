# barber-booking

Step 1 – Define the Users

Our application has three types of users.
1. Admin (Application Owner)

This is you.

The admin controls the entire system.

The admin can:

Log in
Create barber accounts
Edit bar

This is where Go starts executing the program.ber information
Delete barber accounts
Suspend or activate barber accounts
View all barbers
View all bookings (optional later)
Manage the whole application

The admin cannot book appointments as an admin.

2. Barber

Each barber has their own account.

A barber can:

Log in
Edit their profile
Upload hairstyle pictures
Set hairstyle prices
Delete hairstyles
Accept bookings
Reject bookings
Chat with customers
View their customers
Set working hours (later)

A barber cannot create another barber.

3. Customer

Customers do not need an account in the first version.

A customer can:

Enter a barber's phone number (or use a link)
View hairstyles
View prices
Choose a date
Choose a time
Book an appointment
Chat with the barber

Later we can decide whether customers should have accounts.

Admin Flow
Open Website
      │
      ▼
Login
      │
      ▼
Dashboard
      │
      ├─────────────► Add Barber
      │
      ├─────────────► Edit Barber
      │
      ├─────────────► Delete Barber
      │
      └─────────────► Logout
      
      Step 2 – Application Flow

Instead of thinking about pages first, think about what each user does.

Admin Flow
Open Website
      │
      ▼
Login
      │
      ▼
Dashboard
      │
      ├─────────────► Add Barber
      │
      ├─────────────► Edit Barber
      │
      ├─────────────► Delete Barber
      │
      └─────────────► Logout
      
      Barber Flow
Open Website
      │
      ▼
Login
      │
      ▼
Dashboard
      │
      ├────────► Add Hairstyle
      │
      ├────────► Upload Image
      │
      ├────────► Set Price
      │
      ├────────► View Bookings
      │
      ├────────► Chat
      │
      └────────► Logout
      
      Customer Flow
Open Barber Page
      │
      ▼
View Hairstyles
      │
      ▼
Choose Hairstyle
      │
      ▼
Choose Date
      │
      ▼
Choose Time
      │
      ▼
Book Appointment
      │
      ▼
Chat With Barber

Step 3 – Pages We Need

This project will eventually have several pages.

Admin Pages
Login

Dashboard

Add Barber

Edit Barber

View Barbers

Barber Pages
Login

Dashboard

Add Hairstyle

Edit Hairstyle

Bookings

Chat

Profile

Customer Pages
Home

Barber Page

Booking Page

Chat

Step 4 – Folder Structure

Before writing code, we decide where every file belongs.

This is the structure I recommend:

barber-booking/
│
├── main.go
│
├── go.mod
│
├── handlers/
│
├── templates/
│
├── static/
│   ├── css/
│   ├── js/
│   └── images/
│
├── uploads/
│
├── models/
│
├── database/
│
├── middleware/
│
├── routes/
│
├── utils/
│
└── README.md

What problem does this solve?
Why are we solving it this way instead of another way?