import{R as M,ba as N,S as u,b3 as b,a as x,m as i,b as T,Z as V,d as D,e as E,h as s,a5 as q,bb as K,aB as Z,W as G,bc as J,bd as Q,be as U,bf as X,q as Y,a2 as oo,u as eo,f as H,af as ro,x as no,n as _,r as to,bg as so,A as d,H as lo,a7 as io,E as ao,L as co,a8 as go,F as ho}from"./index-B6oWHZbC.js";function uo(r){const{lineHeight:o,borderRadius:a,fontWeightStrong:C,baseColor:l,dividerColor:f,actionColor:S,textColor1:g,textColor2:t,closeColorHover:h,closeColorPressed:v,closeIconColor:m,closeIconColorHover:p,closeIconColorPressed:n,infoColor:e,successColor:I,warningColor:z,errorColor:y,fontSize:P}=r;return Object.assign(Object.assign({},N),{fontSize:P,lineHeight:o,titleFontWeight:C,borderRadius:a,border:`1px solid ${f}`,color:S,titleTextColor:g,iconColor:t,contentTextColor:t,closeBorderRadius:a,closeColorHover:h,closeColorPressed:v,closeIconColor:m,closeIconColorHover:p,closeIconColorPressed:n,borderInfo:`1px solid ${u(l,b(e,{alpha:.25}))}`,colorInfo:u(l,b(e,{alpha:.08})),titleTextColorInfo:g,iconColorInfo:e,contentTextColorInfo:t,closeColorHoverInfo:h,closeColorPressedInfo:v,closeIconColorInfo:m,closeIconColorHoverInfo:p,closeIconColorPressedInfo:n,borderSuccess:`1px solid ${u(l,b(I,{alpha:.25}))}`,colorSuccess:u(l,b(I,{alpha:.08})),titleTextColorSuccess:g,iconColorSuccess:I,contentTextColorSuccess:t,closeColorHoverSuccess:h,closeColorPressedSuccess:v,closeIconColorSuccess:m,closeIconColorHoverSuccess:p,closeIconColorPressedSuccess:n,borderWarning:`1px solid ${u(l,b(z,{alpha:.33}))}`,colorWarning:u(l,b(z,{alpha:.08})),titleTextColorWarning:g,iconColorWarning:z,contentTextColorWarning:t,closeColorHoverWarning:h,closeColorPressedWarning:v,closeIconColorWarning:m,closeIconColorHoverWarning:p,closeIconColorPressedWarning:n,borderError:`1px solid ${u(l,b(y,{alpha:.25}))}`,colorError:u(l,b(y,{alpha:.08})),titleTextColorError:g,iconColorError:y,contentTextColorError:t,closeColorHoverError:h,closeColorPressedError:v,closeIconColorError:m,closeIconColorHoverError:p,closeIconColorPressedError:n})}const bo={common:M,self:uo},fo=x("alert",`
 line-height: var(--n-line-height);
 border-radius: var(--n-border-radius);
 position: relative;
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-color);
 text-align: start;
 word-break: break-word;
`,[i("border",`
 border-radius: inherit;
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 transition: border-color .3s var(--n-bezier);
 border: var(--n-border);
 pointer-events: none;
 `),T("closable",[x("alert-body",[i("title",`
 padding-right: 24px;
 `)])]),i("icon",{color:"var(--n-icon-color)"}),x("alert-body",{padding:"var(--n-padding)"},[i("title",{color:"var(--n-title-text-color)"}),i("content",{color:"var(--n-content-text-color)"})]),V({originalTransition:"transform .3s var(--n-bezier)",enterToProps:{transform:"scale(1)"},leaveToProps:{transform:"scale(0.9)"}}),i("icon",`
 position: absolute;
 left: 0;
 top: 0;
 align-items: center;
 justify-content: center;
 display: flex;
 width: var(--n-icon-size);
 height: var(--n-icon-size);
 font-size: var(--n-icon-size);
 margin: var(--n-icon-margin);
 `),i("close",`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 position: absolute;
 right: 0;
 top: 0;
 margin: var(--n-close-margin);
 `),T("show-icon",[x("alert-body",{paddingLeft:"calc(var(--n-icon-margin-left) + var(--n-icon-size) + var(--n-icon-margin-right))"})]),T("right-adjust",[x("alert-body",{paddingRight:"calc(var(--n-close-size) + var(--n-padding) + 2px)"})]),x("alert-body",`
 border-radius: var(--n-border-radius);
 transition: border-color .3s var(--n-bezier);
 `,[i("title",`
 transition: color .3s var(--n-bezier);
 font-size: 16px;
 line-height: 19px;
 font-weight: var(--n-title-font-weight);
 `,[D("& +",[i("content",{marginTop:"9px"})])]),i("content",{transition:"color .3s var(--n-bezier)",fontSize:"var(--n-font-size)"})]),i("icon",{transition:"color .3s var(--n-bezier)"})]),vo=Object.assign(Object.assign({},H.props),{title:String,showIcon:{type:Boolean,default:!0},type:{type:String,default:"default"},bordered:{type:Boolean,default:!0},closable:Boolean,onClose:Function,onAfterLeave:Function,onAfterHide:Function}),Co=E({name:"Alert",inheritAttrs:!1,props:vo,slots:Object,setup(r){const{mergedClsPrefixRef:o,mergedBorderedRef:a,inlineThemeDisabled:C,mergedRtlRef:l}=eo(r),f=H("Alert","-alert",fo,bo,r,o),S=ro("Alert",l,o),g=_(()=>{const{common:{cubicBezierEaseInOut:n},self:e}=f.value,{fontSize:I,borderRadius:z,titleFontWeight:y,lineHeight:P,iconSize:$,iconMargin:R,iconMarginRtl:W,closeIconSize:w,closeBorderRadius:A,closeSize:B,closeMargin:k,closeMarginRtl:L,padding:j}=e,{type:c}=r,{left:F,right:O}=so(R);return{"--n-bezier":n,"--n-color":e[d("color",c)],"--n-close-icon-size":w,"--n-close-border-radius":A,"--n-close-color-hover":e[d("closeColorHover",c)],"--n-close-color-pressed":e[d("closeColorPressed",c)],"--n-close-icon-color":e[d("closeIconColor",c)],"--n-close-icon-color-hover":e[d("closeIconColorHover",c)],"--n-close-icon-color-pressed":e[d("closeIconColorPressed",c)],"--n-icon-color":e[d("iconColor",c)],"--n-border":e[d("border",c)],"--n-title-text-color":e[d("titleTextColor",c)],"--n-content-text-color":e[d("contentTextColor",c)],"--n-line-height":P,"--n-border-radius":z,"--n-font-size":I,"--n-title-font-weight":y,"--n-icon-size":$,"--n-icon-margin":R,"--n-icon-margin-rtl":W,"--n-close-size":B,"--n-close-margin":k,"--n-close-margin-rtl":L,"--n-padding":j,"--n-icon-margin-left":F,"--n-icon-margin-right":O}}),t=C?no("alert",_(()=>r.type[0]),g,r):void 0,h=to(!0),v=()=>{const{onAfterLeave:n,onAfterHide:e}=r;n&&n(),e&&e()};return{rtlEnabled:S,mergedClsPrefix:o,mergedBordered:a,visible:h,handleCloseClick:()=>{var n;Promise.resolve((n=r.onClose)===null||n===void 0?void 0:n.call(r)).then(e=>{e!==!1&&(h.value=!1)})},handleAfterLeave:()=>{v()},mergedTheme:f,cssVars:C?void 0:g,themeClass:t==null?void 0:t.themeClass,onRender:t==null?void 0:t.onRender}},render(){var r;return(r=this.onRender)===null||r===void 0||r.call(this),s(oo,{onAfterLeave:this.handleAfterLeave},{default:()=>{const{mergedClsPrefix:o,$slots:a}=this,C={class:[`${o}-alert`,this.themeClass,this.closable&&`${o}-alert--closable`,this.showIcon&&`${o}-alert--show-icon`,!this.title&&this.closable&&`${o}-alert--right-adjust`,this.rtlEnabled&&`${o}-alert--rtl`],style:this.cssVars,role:"alert"};return this.visible?s("div",Object.assign({},q(this.$attrs,C)),this.closable&&s(K,{clsPrefix:o,class:`${o}-alert__close`,onClick:this.handleCloseClick}),this.bordered&&s("div",{class:`${o}-alert__border`}),this.showIcon&&s("div",{class:`${o}-alert__icon`,"aria-hidden":"true"},Z(a.icon,()=>[s(G,{clsPrefix:o},{default:()=>{switch(this.type){case"success":return s(X,null);case"info":return s(U,null);case"warning":return s(Q,null);case"error":return s(J,null);default:return null}}})])),s("div",{class:[`${o}-alert-body`,this.mergedBordered&&`${o}-alert-body--bordered`]},Y(a.header,l=>{const f=l||this.title;return f?s("div",{class:`${o}-alert-body__title`},f):null}),a.default&&s("div",{class:`${o}-alert-body__content`},a))):null}})}}),po=E({__name:"PermissionWarning",props:{text:{}},setup(r){return(o,a)=>(lo(),io(ho(Co),{type:"warning",bordered:!1,style:{"margin-bottom":"12px"}},{default:ao(()=>[co(go(r.text),1)]),_:1}))}});export{po as _};
