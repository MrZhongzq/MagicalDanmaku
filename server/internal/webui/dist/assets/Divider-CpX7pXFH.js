import{a as f,Y as c,m as s,b as n,e as m,h as a,a0 as p,u as b,f as v,x,n as C,bo as $}from"./index-DnzD1cCi.js";function z(i,t="default",o=[]){const l=i.$slots[t];return l===void 0?o:l()}const w=f("divider",`
 position: relative;
 display: flex;
 width: 100%;
 box-sizing: border-box;
 font-size: 16px;
 color: var(--n-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
`,[c("vertical",`
 margin-top: 24px;
 margin-bottom: 24px;
 `,[c("no-title",`
 display: flex;
 align-items: center;
 `)]),s("title",`
 display: flex;
 align-items: center;
 margin-left: 12px;
 margin-right: 12px;
 white-space: nowrap;
 font-weight: var(--n-font-weight);
 `),n("title-position-left",[s("line",[n("left",{width:"28px"})])]),n("title-position-right",[s("line",[n("right",{width:"28px"})])]),n("dashed",[s("line",`
 background-color: #0000;
 height: 0px;
 width: 100%;
 border-style: dashed;
 border-width: 1px 0 0;
 `)]),n("vertical",`
 display: inline-block;
 height: 1em;
 margin: 0 8px;
 vertical-align: middle;
 width: 1px;
 `),s("line",`
 border: none;
 transition: background-color .3s var(--n-bezier), border-color .3s var(--n-bezier);
 height: 1px;
 width: 100%;
 margin: 0;
 `),c("dashed",[s("line",{backgroundColor:"var(--n-color)"})]),n("dashed",[s("line",{borderColor:"var(--n-color)"})]),n("vertical",{backgroundColor:"var(--n-color)"})]),_=Object.assign(Object.assign({},v.props),{titlePlacement:{type:String,default:"center"},dashed:Boolean,vertical:Boolean}),k=m({name:"Divider",props:_,setup(i){const{mergedClsPrefixRef:t,inlineThemeDisabled:o}=b(i),d=v("Divider","-divider",w,$,i,t),l=C(()=>{const{common:{cubicBezierEaseInOut:e},self:{color:h,textColor:g,fontWeight:u}}=d.value;return{"--n-bezier":e,"--n-color":h,"--n-text-color":g,"--n-font-weight":u}}),r=o?x("divider",void 0,l,i):void 0;return{mergedClsPrefix:t,cssVars:o?void 0:l,themeClass:r==null?void 0:r.themeClass,onRender:r==null?void 0:r.onRender}},render(){var i;const{$slots:t,titlePlacement:o,vertical:d,dashed:l,cssVars:r,mergedClsPrefix:e}=this;return(i=this.onRender)===null||i===void 0||i.call(this),a("div",{role:"separator",class:[`${e}-divider`,this.themeClass,{[`${e}-divider--vertical`]:d,[`${e}-divider--no-title`]:!t.default,[`${e}-divider--dashed`]:l,[`${e}-divider--title-position-${o}`]:t.default&&o}],style:r},d?null:a("div",{class:`${e}-divider__line ${e}-divider__line--left`}),!d&&t.default?a(p,null,a("div",{class:`${e}-divider__title`},this.$slots),a("div",{class:`${e}-divider__line ${e}-divider__line--right`})):null)}});export{k as N,z as g};
