GearStack Frontend (Prototype)

This is the "Thick Client" prototype for GearStack, a telescoping inventory management system for backpacking and cycling gear. It implements a game-like UI (similar to OSRS/Tarkov) using React and Tailwind CSS.

🌟 Core Features

1. Telescoping Inventory System

Unlike standard flat lists, this inventory is recursive:

Root: Represents the user (slots for Worn Gear, Pack, etc.).

Nesting: Items can provide "Slots". (e.g., A "Backpack" provides a "Main Compartment" and "Side Pockets").

Zoom: Users can drill down into any container to manage its contents specifically.

2. 4x8 Interactive Grid

Fixed Layout: Renders a 32-cell grid (4 columns x 8 rows) for consistent "Tetris-style" organization.

Drag & Drop: Native HTML5 DnD implementation allows users to rearrange items within the grid.

Persistence: The grid position (index 0-31) of every item is tracked in the state, even when zooming in/out.

3. Smart Stats

Recursive Calculation: Weights and costs are summed from the bottom up (e.g., pills inside a bottle inside a ditty bag inside a pack).

Base Weight vs. Total: Automatically excludes items marked consumable: true (food, water, fuel) from the Base Weight calculation—a critical metric for hikers.

4. The Garage (Database)

Master List: A searchable database of all available gear.

Smart Import (Prototype): A simulation of the "Link Unfurling" feature where pasting a URL (e.g., Backcountry.com) scrapes item details.

🛠 Tech Stack

Framework: React (Vite)

Styling: Tailwind CSS

Icons: Lucide-React

State: Local React State (Migrating to React Query + Golang in Phase 2)

🚀 Running Locally

cd frontend
npm install
npm run dev -- --host

