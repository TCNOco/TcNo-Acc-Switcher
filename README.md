
<p align="center">
  <a href="https://tcno.co/">
    <img src="/other/img/Banner.png"></a>
</p>
<p align="center">
  <img alt="GitHub All Releases" src="https://img.shields.io/github/downloads/TCNOco/TcNo-Acc-Switcher/total?logo=GitHub&style=flat-square">
  <a href="https://tcno.co/">
    <img alt="Website" src="/other/img/web.svg" height=20></a>
  <a href="https://s.tcno.co/AccSwitcherDiscord">
    <img alt="Discord server" src="https://img.shields.io/discord/217649733915770880?label=Discord&logo=discord&style=flat-square"></a>
  <a href="https://twitter.com/TCNOco">
    <img alt="Twitter" src="https://img.shields.io/twitter/follow/TCNOco?label=Follow%20%40TCNOCo&logo=Twitter&style=flat-square"></a>
  <img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/TCNOco/TcNo-Acc-Switcher?logo=GitHub&style=flat-square">
  <img alt="GitHub repo size" src="https://img.shields.io/github/repo-size/TCNOco/TcNo-Acc-Switcher?logo=GitHub&style=flat-square">
																     <a title="Crowdin" target="_blank" href="https://crowdin.com/project/tcno-account-switcher"><img src="https://img.shields.io/badge/Seeking-localisers-blue.svg?style=flat-square"></a>
</p>
<p align="center">
  <a href="https://patreon.com/TroubleChute">
    <img alt="Patreon" src="https://img.shields.io/badge/Patreon-F96854?style=for-the-badge&logo=patreon&logoColor=white"></a>
  <a href="https://ko-fi.com/art?=redirect">
    <img alt="Ko-Fi" src="https://img.shields.io/badge/Ko--fi-F16061?style=for-the-badge&logo=ko-fi&logoColor=white"></a>
</p> 

<p align="center"><a target="_blank" href="https://github.com/TCNOco/TcNo-Acc-Switcher/releases/latest">
  <img alt="Download latest" src="/other/img/DownloadLatestNEW.png" height=70"></a>
  <a target="_blank" href="https://github.com/TCNOco/TcNo-Acc-Switcher/wiki">
  <img alt="More info" src="/other/img/WikiButton.png" height=70"></a>
</p>
<p align="center"><a target="_blank" href="https://github.com/TCNOco/TcNo-Acc-Switcher-Themes">
  <img alt="Themes" src="/other/img/Themes.png" height=70"></a>
  <a target="_blank" href="https://github.com/TCNOco/TcNo-Acc-Switcher/wiki/Frequently-Asked-Questions">
  <img alt="Themes" src="/other/img/FAQButton.png" height=70"></a>
</p>
  
**A Superfast open-source account switcher for Steam, Battle.net, Epic Games, Origin, Riot Games, Ubisoft, and more**.
							     
This is an experimental rebuild in Go for a more maintainable, faster, monolithic structure (but anything could change through this rewrite).

Find the current live version [here on GitHub](https://github.com/TCNOco/TcNo-Acc-Switcher)

### Why?

This project started in C++ as a CLI application, moved to C# WPF which was great... But XAML requiring a complete component rewrite for custom styles and more was more tiring. HTML/CSS is a great solution, which is when it switched to Blazor. I was working towards better memory management and compatability with .NET MAUI with Singletons and the rest, which was a HUGE amount of effort just for my efforts to end with not being able to drag and drop items on the page. This was a Microsoft issue, and not solved for months. I haven't looked again, but I wouldn't be surprised if it's still not fixed.

This project started very long ago and through so many rewrites has ended up with a little sphaghetti code where errors are possibly unhandled and more. Go, like Rust and other languages forces you to to handle errors more than being built around ignoring them.

### Go?

Yeah. Better performance, compiled instead of JIT and cross-platform compatibility. While C# offers some cross-platform compatability, I'd prefer a MUCH smaller package without a copy of .NET runtime included. While I fear Go might end up a little verbose with the error handling, I would hope this is a good idea for more stable, faster performance.

### AI?

The TcNo Account Switcher started off as one of my first big projects. Back when StackOverflow was the go-to. Vibe coding is great for throw away projects or small tools, but this isn't one of them. The most I'll be using is Ask mode. This project focuses on security, stability and performance.


#### Disclaimer

```
All trademarks and materials are the property of their respective owners and their licensors. This project is not affiliated
with any companies referenced. This is not "Official" software or related to any companies mentioned. All it does is let you
move your files around on your computer the same way you can. The use of names, icons and trademarks does not indicate
endorsement of the trademark holder by this project or its creators, nor vice versa. They are only used to visually indicate
which programs this project interacts with easily to the end-user.

By enabling optional features that scrape the web for publically available information (such as limited game/profile statistics
and other data), you understand and accept full responsibility for doing so on your own volition. If you appreciate accurate
information, support the services providing it directly. The information collected is incredibly limited and is no replacement
or competitor for sites scraped.

I am not responsible for the contents of external links.
For the rest of the disclaimer, refer to the License (GNU General Public License v3.0) file:
https://github.com/TCNOCo/TcNo-Acc-Switcher/blob/master/LICENSE - See sections like 15, 16 and 17, as well as GitHub's
'simplification' at the top of the above website.
```

#### [Privacy Policy](https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/PRIVACY.md)

Additional license information for included NuGet packages and other parts of code can be found in: [HERE](https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/TcNo-Acc-Switcher-Server/Additional%20Licenses.txt) `TcNo-Acc-Switcher-Server/Additional Licenses.txt`, and are copied to the build directory, as well as distributed with release versions of this software.
