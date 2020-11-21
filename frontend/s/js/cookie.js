'use strict';

window.addEventListener('load', function() {
  document.getElementsByClassName('ok')[0].onclick = function() {
      document.cookie= "HansaAuthNAccept=true;"
      document.getElementsByClassName('ok')[0].disabled = true
      document.getElementsByClassName('ok')[0].style['background-color'] = 'gray'
      location.reload()
  }
})
