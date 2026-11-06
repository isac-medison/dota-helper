# Dota 2 Helper

<p align="center">
  <img src="appicon.png" alt="Dota 2 Helper App Icon" width="200"/>
</p>

Dota 2 Helper is a comprehensive mobile companion application designed to elevate your gameplay. It provides real-time drafting analysis, meta statistics and in-game timers to help you make informed decisions on the battlefield.

## Features

* **Smart Draft Analyzer:** Get real-time counter-pick suggestions and synergy analysis during the drafting phase.
* **Hero & Player Stats:** Access detailed win rates, pick rates, and your personal match history via the Steam Web API.

## Tech Stack

* **Frontend:** React (via Wails)
* **Backend:** Go
* **APIs:** OpenDota API, Steam Web API

## Installation

### Prerequisites
* [Go](https://go.dev/) (v1.26 or higher)
* [Node.js](https://nodejs.org/) (v18 or higher)
* [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### Setup Steps

1. **Clone the repository**

2. **Install Flutter dependencies**

3. **Configure the environment variables**
   Create a `.env` file in the root directory and add your API keys:

4. **Run the app in development mode**
   ```bash
   wails3 dev
   ```

## License

This project is licensed under the MIT License. See the `LICENSE` file for details. Dota 2 is a registered trademark of Valve Corporation. This project is not affiliated with Valve Corporation.